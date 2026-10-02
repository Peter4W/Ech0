// SPDX-License-Identifier: AGPL-3.0-or-later
// top 模块入口：零依赖边车，反向代理 Ech0 并注入"动态置顶"能力。
// Ech0 本体代码不做任何修改，全部实现均在 top/ 目录内。

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

//go:embed all:static
var staticFS embed.FS

type Config struct {
	Addr     string
	Upstream string
	DataDir  string
	MaxPins  int
}

func loadConfig() Config {
	c := Config{
		Addr:     getenv("TOP_ADDR", ":6278"),
		Upstream: getenv("TOP_UPSTREAM", "http://localhost:6277"),
		DataDir:  getenv("TOP_DATA", "./data"),
		MaxPins:  10,
	}
	if v := getenv("TOP_MAX_PINS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxPins = n
		}
	}
	return c
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()

	store, err := NewPinStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("top: init pin store: %v", err)
	}

	app := NewApp(cfg, store)

	mux := http.NewServeMux()

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("top: static fs: %v", err)
	}
	mux.Handle("/top/static/", http.StripPrefix("/top/static/", http.FileServer(http.FS(staticSub))))
	app.registerAPI(mux)
	mux.Handle("/", app)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           withLogging(mux),
		ReadHeaderTimeout: 15 * time.Second,
	}

	log.Printf("top: sidecar listening on %s -> upstream %s", cfg.Addr, cfg.Upstream)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("top: server: %v", err)
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush 透传，保证 SSE/流式响应正常。
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
