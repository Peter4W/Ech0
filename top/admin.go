// SPDX-License-Identifier: AGPL-3.0-or-later
// top 模块：置顶管理接口。鉴权复用 Ech0 —— 携令牌回源 /api/user 校验 is_admin。

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type apiResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(apiResponse{Code: code, Msg: msg, Data: data})
}

func (a *App) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("/top/api/pins", a.handlePins)
	mux.HandleFunc("/top/api/pins/", a.handlePinItem) // /top/api/pins/{id} 与 /reorder
}

// handlePins: GET 列表 / POST 新增置顶
func (a *App) handlePins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 1, "ok", map[string]any{"pins": a.store.List()})
	case http.MethodPost:
		if !a.requireAdmin(w, r) {
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, 0, "invalid body", nil)
			return
		}
		id := strings.TrimSpace(body.ID)
		if id == "" {
			writeJSON(w, 0, "id required", nil)
			return
		}
		if len(a.store.List()) >= a.cfg.MaxPins && !a.store.Has(id) {
			writeJSON(w, 0, "pin limit exceeded", nil)
			return
		}
		// 校验动态真实存在（携带操作者令牌）。
		if _, ok := a.fetchPinnedEcho(r, id); !ok {
			writeJSON(w, 0, "echo not found or not visible", nil)
			return
		}
		if err := a.store.Add(id); err != nil {
			writeJSON(w, 0, "store error: "+err.Error(), nil)
			return
		}
		writeJSON(w, 1, "pinned", map[string]any{"pins": a.store.List()})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePinItem 路由：/top/api/pins/reorder (PUT) 与 /top/api/pins/{id} (DELETE)
func (a *App) handlePinItem(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/top/api/pins/")
	if suffix == "reorder" {
		if r.Method != http.MethodPut {
			w.Header().Set("Allow", "PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !a.requireAdmin(w, r) {
			return
		}
		var body struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, 0, "invalid body", nil)
			return
		}
		if err := a.store.Reorder(body.IDs); err != nil {
			writeJSON(w, 0, err.Error(), nil)
			return
		}
		writeJSON(w, 1, "reordered", map[string]any{"pins": a.store.List()})
		return
	}

	id := strings.TrimSpace(suffix)
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.requireAdmin(w, r) {
		return
	}
	if err := a.store.Remove(id); err != nil {
		writeJSON(w, 0, "store error: "+err.Error(), nil)
		return
	}
	writeJSON(w, 1, "unpinned", map[string]any{"pins": a.store.List()})
}

// requireAdmin 校验调用者身份，仅管理员放行；失败时写出响应。
func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		writeJSON(w, 0, "login required", nil)
		return false
	}
	if !a.isCallerAdmin(r) {
		writeJSON(w, 0, "admin only", nil)
		return false
	}
	return true
}

// isCallerAdmin 携调用者令牌回源 /api/user，返回是否管理员。
func (a *App) isCallerAdmin(r *http.Request) bool {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return false
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		strings.TrimRight(a.cfg.Upstream, "/")+"/api/user", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", auth)
	resp, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return false
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			IsAdmin bool `json:"is_admin"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return false
	}
	return env.Code == 1 && env.Data.IsAdmin
}
