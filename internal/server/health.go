package server

import (
	"net/http"
	"os/exec"

	"github.com/kayushkin/llm-bridge/msg"
)

type HealthResponse struct {
	Status   string         `json:"status"`
	Bridges  []BridgeStatus `json:"bridges"`
	Sessions SessionCounts  `json:"sessions"`
}

type BridgeStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
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
	bridges := s.discoverBridges()
	counts := s.sessionCounts()

	status := "ok"
	if counts.Running == 0 && !anyAvailable(bridges) {
		status = "degraded"
	}

	writeJSON(w, HealthResponse{
		Status:   status,
		Bridges:  bridges,
		Sessions: counts,
	})
}

func (s *Server) handleBridges(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.discoverBridges())
}

func (s *Server) discoverBridges() []BridgeStatus {
	var bridges []BridgeStatus

	for _, p := range []msg.Provider{msg.ProviderAnthropic, msg.ProviderOpenAI, msg.ProviderGemini, msg.ProviderOpenRouter} {
		bridges = append(bridges, BridgeStatus{
			Name:      string(p),
			Type:      "api",
			Available: true,
		})
	}

	for h, bin := range harnessBinaries {
		path, err := exec.LookPath(bin)
		bridges = append(bridges, BridgeStatus{
			Name:      string(h),
			Type:      "harness",
			Available: err == nil,
			Binary:    path,
		})
	}

	return bridges
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

func anyAvailable(bridges []BridgeStatus) bool {
	for _, b := range bridges {
		if b.Available {
			return true
		}
	}
	return false
}
