package server

import (
	"net/http"

	"github.com/kayushkin/llm-bridge/internal/harness"
	"github.com/kayushkin/llm-bridge/msg"
)

type HealthResponse struct {
	Status    string          `json:"status"`
	Harnesses []HarnessStatus `json:"harnesses"`
	Sessions  SessionCounts   `json:"sessions"`
}

type HarnessStatus struct {
	Name         string   `json:"name"`
	Available    bool     `json:"available"`
	Binary       string   `json:"binary,omitempty"`
	Capabilities []string `json:"capabilities"`
}

// harnessCapabilities defines what features each harness supports.
var harnessCapabilities = map[msg.Harness][]string{
	msg.HarnessClaudeCode: {"compact", "fork", "model", "effort", "tools", "budget", "system_prompt"},
	msg.HarnessCodex:      {"model"},
	msg.HarnessOpenClaw:   {"compact", "model", "effort"},
	msg.HarnessInber:      {"model"},
	msg.HarnessHermes:     {"model"},
}

type SessionCounts struct {
	Running   int `json:"running"`
	Idle      int `json:"idle"`
	Completed int `json:"completed"`
}

var allHarnesses = []msg.Harness{
	msg.HarnessClaudeCode,
	msg.HarnessCodex,
	msg.HarnessOpenClaw,
	msg.HarnessInber,
	msg.HarnessHermes,
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
	var statuses []HarnessStatus
	for _, h := range allHarnesses {
		path, available := harness.Available(h)
		caps := harnessCapabilities[h]
		if caps == nil {
			caps = []string{}
		}
		statuses = append(statuses, HarnessStatus{
			Name:         string(h),
			Available:    available,
			Binary:       path,
			Capabilities: caps,
		})
	}
	return statuses
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
