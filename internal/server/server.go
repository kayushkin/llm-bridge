package server

import (
	"encoding/json"
	"net/http"

	"github.com/kayushkin/llm-bridge/internal/store"
)

type Server struct {
	mux   *http.ServeMux
	store *store.Store
}

func New(s *store.Store) *Server {
	srv := &Server{
		mux:   http.NewServeMux(),
		store: s,
	}
	srv.routes()
	return srv
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /harnesses", s.handleHarnesses)
	s.mux.HandleFunc("GET /sessions", s.handleListSessions)
	s.mux.HandleFunc("POST /sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /sessions/{id}/stop", s.handleStopSession)
	s.mux.HandleFunc("POST /sessions/{id}/send", s.handleSendMessage)
	s.mux.HandleFunc("GET /sessions/{id}/events", s.handleSessionEvents)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
