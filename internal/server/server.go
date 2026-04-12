package server

import (
	"encoding/json"
	"net/http"

	"github.com/kayushkin/llm-bridge/internal/harness"
	"github.com/kayushkin/llm-bridge/internal/store"
)

type Server struct {
	mux     *http.ServeMux
	store   *store.Store
	harness *harness.Manager
}

func New(st *store.Store) *Server {
	srv := &Server{
		mux:     http.NewServeMux(),
		store:   st,
		harness: harness.NewManager(st),
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
	s.mux.HandleFunc("POST /sessions/{id}/send", s.handleSendMessage)
	s.mux.HandleFunc("GET /sessions/{id}/events", s.handleSessionEvents)

	s.mux.HandleFunc("GET /sessions/{id}/messages", s.handleSessionMessages)
	s.mux.HandleFunc("GET /sessions/{id}/history", s.handleSessionHistory)
	s.mux.HandleFunc("POST /sessions/{id}/interrupt", s.handleInterruptSession)
	s.mux.HandleFunc("POST /sessions/{id}/resume", s.handleResumeSession)
	s.mux.HandleFunc("POST /sessions/{id}/stop", s.handleStopSession)
	s.mux.HandleFunc("POST /sessions/{id}/compact", s.handleCompactSession)
	s.mux.HandleFunc("POST /sessions/{id}/fork", s.handleForkSession)
	s.mux.HandleFunc("POST /sessions/{id}/config", s.handleConfigSession)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
