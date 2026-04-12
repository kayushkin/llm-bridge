package server

import (
	"net/http"
	"os/exec"

	"github.com/kayushkin/llm-bridge/msg"
)

type HealthResponse struct {
	Status    string          `json:"status"`
	Harnesses []HarnessStatus `json:"harnesses"`
	Sessions  SessionCounts   `json:"sessions"`
}

type HarnessStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Binary    string `json:"binary,omitempty"`
}

type SessionCounts struct {
	Running   int `json:"running"`
	Idle      int `json:"idle"`
	Completed int `json:"completed"`
}

var harnessBinaries = map[msg.Harness]string{
	msg.HarnessClaudeCode: "llm-bridge-claudecode",
	msg.HarnessCodex:      "llm-bridge-codex",
	msg.HarnessOpenClaw:   "llm-bridge-openclaw",
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	harnesses := s.discoverHarnesses()
	counts := s.sessionCounts()

	status := "ok"
	if counts.Running == 0 && !anyAvailable(harnesses) {
		status = "degraded"
	}

	writeJSON(w, HealthResponse{
		Status:    status,
		Harnesses: harnesses,
		Sessions:  counts,
	})
}

func (s *Server) handleHarnesses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.discoverHarnesses())
}

func (s *Server) discoverHarnesses() []HarnessStatus {
	var harnesses []HarnessStatus
	for h, bin := range harnessBinaries {
		path, err := exec.LookPath(bin)
		harnesses = append(harnesses, HarnessStatus{
			Name:      string(h),
			Available: err == nil,
			Binary:    path,
		})
	}
	return harnesses
}

func (s *Server) sessionCounts() SessionCounts {
	var counts SessionCounts

	if sessions, err := s.store.ListSessionsByState(string(msg.SessionRunning)); err == nil {
		counts.Running = len(sessions)
	}
	if sessions, err := s.store.ListSessionsByState(string(msg.SessionIdle)); err == nil {
		counts.Idle = len(sessions)
	}
	if sessions, err := s.store.ListSessionsByState(string(msg.SessionCompleted)); err == nil {
		counts.Completed = len(sessions)
	}

	return counts
}

func anyAvailable(harnesses []HarnessStatus) bool {
	for _, h := range harnesses {
		if h.Available {
			return true
		}
	}
	return false
}
