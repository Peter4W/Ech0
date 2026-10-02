// SPDX-License-Identifier: AGPL-3.0-or-later
// top 模块：动态列表响应重排（置顶前置、去重、打标）与 HTML 资源注入。

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// envelope 对应 Ech0 的统一响应包：{"code":..,"msg":..,"data":..}
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type echoPageData struct {
	Total int64            `json:"total"`
	Items []map[string]any `json:"items"`
}

// rewriteEchoList 在数据层重排列表，返回是否完成改写。
func (a *App) rewriteEchoList(r *http.Request, resp *http.Response, reqBody []byte) (bool, error) {
	body, err := readLimited(resp)
	if err != nil {
		return false, err
	}
	// 上游 body 已被读空：无论是否重写，都用字节重建，供代理继续拷贝。
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("parse envelope: %w", err)
	}
	if env.Code != 1 || len(env.Data) == 0 {
		return false, nil
	}

	var page echoPageData
	if err := json.Unmarshal(env.Data, &page); err != nil {
		return false, fmt.Errorf("parse page data: %w", err)
	}

	pinIDs := a.store.List()
	if len(pinIDs) == 0 {
		return false, nil
	}

	// 请求体中读取分页（缺省 page=1, pageSize=10，与上游一致）。
	pageNo := 1
	if len(bytes.TrimSpace(reqBody)) > 0 {
		var q struct {
			Page int `json:"page"`
		}
		if err := json.Unmarshal(reqBody, &q); err == nil && q.Page > 0 {
			pageNo = q.Page
		}
	}

	// 服务端回源取置顶动态详情；携带调用者令牌，私密置顶仅其所有者/管理员可见。
	pinned := make([]map[string]any, 0, len(pinIDs))
	pinSet := make(map[string]bool, len(pinIDs))
	for _, id := range pinIDs {
		item, ok := a.fetchPinnedEcho(r, id)
		if !ok {
			continue // 已删除/转私密/不可见：下面统一清理
		}
		item["top_pinned"] = true
		pinned = append(pinned, item)
		pinSet[id] = true
	}
	// 僵尸置顶清理：仅管理员请求时执行（私密置顶对匿名不可见但仍合法，
	// 管理员可见全部，此时拉不到的才是真删除）。
	if len(pinSet) != len(pinIDs) && r.Header.Get("Authorization") != "" && a.isCallerAdmin(r) {
		// 记录本次缺失；仅"连续 2 次缺失"才真正清理，抵御瞬时回源错误。
		gone := a.recordMisses(pinIDs, pinSet)
		if len(gone) > 0 {
			valid := pinSet
			for _, id := range gone {
				valid[id] = false
			}
			if changed, err := a.store.Prune(valid); err != nil {
				return false, fmt.Errorf("prune pins: %w", err)
			} else if changed {
				a.clearMisses(a.store.List())
				// 清理后本次返回原响应，下次请求即为干净结果。
				return false, nil
			}
		}
	}

	// 从普通列表中剔除与置顶重复的项，并给混在列表中的置顶项打标。
	normal := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		id, _ := item["id"].(string)
		if pinSet[id] {
			continue // 已在置顶区出现，避免重复
		}
		normal = append(normal, item)
	}

	items := normal
	if pageNo == 1 {
		items = append(pinned, normal...)
	}
	page.Items = items
	// total 保持上游总数：置顶项仍是集合一员，分页页数不被破坏。

	newData, err := json.Marshal(page)
	if err != nil {
		return false, err
	}
	env.Data = newData
	out, err := json.Marshal(env)
	if err != nil {
		return false, err
	}

	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(out)))
	resp.Header.Set("Top-Rewritten", "1")
	resp.Header.Del("Content-Encoding")
	return true, nil
}

// fetchPinnedEcho 边车回源 GET /api/echo/{id}，解析统一响应包取出动态对象。
func (a *App) fetchPinnedEcho(r *http.Request, id string) (map[string]any, bool) {
	u := strings.TrimRight(a.cfg.Upstream, "/") + "/api/echo/" + id
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, false
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, false
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Code != 1 {
		return nil, false
	}
	var item map[string]any
	if err := json.Unmarshal(env.Data, &item); err != nil || item == nil {
		return nil, false
	}
	if _, ok := item["id"].(string); !ok {
		return nil, false
	}
	return item, true
}

func readLimited(resp *http.Response) ([]byte, error) {
	if resp.Body == nil {
		return nil, fmt.Errorf("empty response body")
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}

var assetTags = []byte(
	`<link rel="stylesheet" href="/top/static/top.css">` +
		`<script src="/top/static/top.js" defer></script>` + "\n")

// injectAssets 向 HTML 页面 head 中注入 top.css/top.js 引用。
func injectAssets(resp *http.Response) error {
	raw, err := readLimited(resp)
	if err != nil {
		return err
	}
	lower := bytes.ToLower(raw)

	idx := bytes.Index(lower, []byte("</head>"))
	if idx >= 0 {
		raw = append(raw[:idx], append(assetTags, raw[idx:]...)...)
	} else {
		// 没有 head 结束标签时插在文档最前。
		raw = append(assetTags, raw...)
	}

	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	resp.ContentLength = int64(len(raw))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(raw)))
	resp.Header.Del("Content-Encoding")
	return nil
}
