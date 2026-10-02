// SPDX-License-Identifier: AGPL-3.0-or-later
// top 模块：反向代理。透传 Ech0 的全部请求，并在响应层完成
// 1) 动态列表置顶重排  2) HTML 页面自动注入 top.js / top.css

package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// App 是整个边车：管理接口之外的请求全部由它代理到上游 Ech0。
type App struct {
	cfg      Config
	store    *PinStore
	upstream *url.URL
	client   *http.Client

	missMu     sync.Mutex
	missStreak map[string]int // 置顶回源连续失败计数，防瞬时错误误删
}

func NewApp(cfg Config, store *PinStore) *App {
	u, err := url.Parse(cfg.Upstream)
	if err != nil {
		log.Fatalf("top: invalid upstream %q: %v", cfg.Upstream, err)
	}
	return &App{
		cfg:      cfg,
		store:    store,
		upstream: u,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		missStreak: map[string]int{},
	}
}

const (
	echoQueryPath = "/api/echo/query"
	maxBodyBytes  = 16 << 20 // 列表/HTML 响应改写上限 16MB
)

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 动态列表接口需要读取请求体中的分页参数，读完后还原供上游使用。
	var reqBody []byte
	if r.Method == http.MethodPost && r.URL.Path == echoQueryPath {
		if r.Body != nil {
			b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, "top: read request body", http.StatusBadGateway)
				return
			}
			reqBody = b
			r.Body = io.NopCloser(bytes.NewReader(b))
			r.ContentLength = int64(len(b))
		}
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = a.upstream.Scheme
			req.URL.Host = a.upstream.Host
			req.Host = a.upstream.Host
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			return a.filterResponse(r, resp, reqBody)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log.Printf("top: upstream error: %v", err)
			http.Error(w, "bad gateway: "+err.Error(), http.StatusBadGateway)
		},
		// SSE/WebSocket 等流式场景由默认透传处理；FlushInterval 保证及时刷新。
		FlushInterval: 100 * time.Millisecond,
	}
	proxy.ServeHTTP(w, r)
}

func (a *App) filterResponse(r *http.Request, resp *http.Response, reqBody []byte) error {
	// 动态列表 JSON：置顶重排。
	if r.Method == http.MethodPost && r.URL.Path == echoQueryPath && resp.StatusCode == http.StatusOK {
		changed, err := a.rewriteEchoList(r, resp, reqBody)
		if err != nil {
			log.Printf("top: rewrite echo list skipped: %v", err)
			return nil // 改写失败不阻断原响应
		}
		if changed {
			return nil
		}
	}

	// HTML 页面：注入 top.js / top.css。
	if isHTML(resp) && r.Method == http.MethodGet && resp.StatusCode == http.StatusOK {
		if err := injectAssets(resp); err != nil {
			log.Printf("top: inject assets skipped: %v", err)
		}
		return nil
	}
	return nil
}

func isHTML(resp *http.Response) bool {
	if resp.Body == nil || resp.Header.Get("Content-Encoding") != "" {
		return false
	}
	ct := resp.Header.Get("Content-Type")
	return strings.Contains(strings.ToLower(ct), "text/html")
}

// recordMisses 累加本轮未回源成功的置顶 ID 的连续失败计数，
// 返回达到 2 次（确认已删除）的 ID；成功取到的 ID 计数清零。
func (a *App) recordMisses(pinIDs []string, pinSet map[string]bool) []string {
	a.missMu.Lock()
	defer a.missMu.Unlock()
	var gone []string
	for _, id := range pinIDs {
		if pinSet[id] {
			delete(a.missStreak, id)
			continue
		}
		a.missStreak[id]++
		log.Printf("top: pinned echo %s upstream miss #%d", id, a.missStreak[id])
		if a.missStreak[id] >= 2 {
			gone = append(gone, id)
		}
	}
	return gone
}

func (a *App) clearMisses(ids []string) {
	a.missMu.Lock()
	defer a.missMu.Unlock()
	a.missStreak = map[string]int{}
	for _, id := range ids {
		delete(a.missStreak, id)
	}
}
