package server

import (
	"net/http"

	"github.com/Whoisvox/godev/shortlink/internal/store"
)

type Server struct {
	store   store.Store
	baseURL string
}

func New(s store.Store, baseURL string) *Server {
	return &Server{store: s, baseURL: baseURL}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", s.shorten)
	mux.HandleFunc("GET /{code}", s.redirect)
	return mux
}
