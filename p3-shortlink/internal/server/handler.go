package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Whoisvox/godev/shortlink/internal/store"
)

// ---- 请求/响应 ----

type shortenReq struct {
	URL string `json:"url"`
}

type shortenResp struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

// ---- POST /api/shorten ----
func (s *Server) shorten(w http.ResponseWriter, r *http.Request) {
	// body 大小限制
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10) // 4KB

	var req shortenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// URL 校验
	raw := strings.TrimSpace(req.URL)
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid url format")
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		writeErr(w, http.StatusBadRequest, "only http/https urls allowed")
		return
	}
	if parsed.Host == "" {
		writeErr(w, http.StatusBadRequest, "url must have a host")
		return
	}

	// 存
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	code, err := s.store.Create(ctx, raw)
	if err != nil {
		slog.Error("store create failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(shortenResp{
		Code:     code,
		ShortURL: s.baseURL + "/" + code,
	})
}

// ---- GET /{code} ----
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	target, err := s.store.Get(ctx, code)
	if err == store.ErrNotFound {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}

	if err != nil {
		slog.Error("store get failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, target, http.StatusFound) // 302
}

// ---- 统一错误响应 ----

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
