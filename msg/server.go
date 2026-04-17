package msg

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Server-managed session
// ──────────────────────────────────────────────────────────────────────────────

// ManagedSession is a session as tracked by llm-bridge-server.
// This is the server's lightweight session management entity, distinct from
// [Session] which models rich agent-level state (usage, tasks, context).
//
// Three IDs track a session through its lifecycle:
//   - BridgeID: server-generated primary key, stable from creation.
//   - HarnessID: the canonical harness session ID (e.g. CC UUID). Empty until
//     the harness reports it on first event.
//   - ClientID: the frontend's correlation key (fe_*). Set at creation,
//     never changes. Used by the frontend to relate the response back to its
//     optimistic UI entry.
type ManagedSession struct {
	BridgeID  string `json:"bridge_id"`
	HarnessID string `json:"harness_id,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	DisplayName     string    `json:"display_name"`
	Harness         Harness   `json:"harness"`
	InstanceID      string    `json:"instance_id,omitempty"`
	State           string    `json:"state"`
	PID             int       `json:"pid,omitempty"`
	AgentID         string    `json:"agent_id,omitempty"`
	SpawnerID       string    `json:"spawner_id,omitempty"`
	ParentID        string          `json:"parent_id,omitempty"`
	HarnessConfig   json.RawMessage `json:"harness_config,omitempty"` // opaque harness-specific config
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Harness registry
// ──────────────────────────────────────────────────────────────────────────────

// HarnessInfo describes a registered harness type and its capabilities.
type HarnessInfo struct {
	Name               string   `json:"name"`
	Label              string   `json:"label"`
	Emoji              string   `json:"emoji"`
	Image              string   `json:"image,omitempty"`
	Available          bool     `json:"available"`
	Binary             string   `json:"binary,omitempty"`
	Capabilities       []string `json:"capabilities"`
	SupportedProviders []string `json:"supported_providers,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Credentials
// ──────────────────────────────────────────────────────────────────────────────

// Credential represents an API credential as exposed by the server.
// Sensitive fields are masked (api_key_masked, token_masked).
type Credential struct {
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	Label        string `json:"label"`
	AuthType     string `json:"auth_type"`
	APIKeyMasked string `json:"api_key_masked,omitempty"`
	TokenMasked  string `json:"token_masked,omitempty"`
	Priority     int    `json:"priority"`
	Enabled      bool   `json:"enabled"`
	ExpiresAt    int64  `json:"expires_at"`
	ErrorCount   int    `json:"error_count,omitempty"`
	LastError    string `json:"last_error,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Bridge preferences
// ──────────────────────────────────────────────────────────────────────────────

// HarnessDefaults stores per-harness configuration defaults.
type HarnessDefaults struct {
	Model         string   `json:"model,omitempty"`
	Effort        string   `json:"effort,omitempty"`
	MaxBudget     *float64 `json:"max_budget,omitempty"`
	DisabledTools []string `json:"disabled_tools,omitempty"`
}

// BridgePrefs stores user preferences for the bridge dashboard.
type BridgePrefs struct {
	LastHarness  string                     `json:"last_harness,omitempty"`
	LastInstanceID string                   `json:"last_instance_id,omitempty"`
	LastSession  map[string]string          `json:"last_session,omitempty"`
	LastInstance  map[string]string          `json:"last_instance,omitempty"`
	Defaults     map[string]HarnessDefaults `json:"defaults,omitempty"`
	SessionNames map[string]string          `json:"session_names,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Materialized messages (from log-store)
// ──────────────────────────────────────────────────────────────────────────────

// MaterializedMessage is a flattened message returned by the messages API.
// Assembled from raw events by log-store: stream deltas are concatenated,
// tool calls are collected, and the final ResultEvent becomes Meta.
type MaterializedMessage struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Thinking  string             `json:"thinking,omitempty"`
	Tools     []MaterializedTool `json:"tools,omitempty"`
	Meta      *ResultEvent       `json:"meta,omitempty"`
	Events    []json.RawMessage  `json:"events,omitempty"`
	Timestamp string             `json:"timestamp"`
	Done      bool               `json:"done"`
}

// MaterializedTool records a tool invocation within a materialized message.
type MaterializedTool struct {
	ToolID string          `json:"tool_id"`
	Tool   string          `json:"tool"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
	Error  bool            `json:"error,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────────────────────────────────────

// HealthResponse is the server's health check response.
type HealthResponse struct {
	Status    string        `json:"status"`
	Harnesses []HarnessInfo `json:"harnesses"`
	Sessions  SessionCounts `json:"sessions"`
}

// SessionCounts is a breakdown of active sessions by state.
type SessionCounts struct {
	Running   int `json:"running"`
	Idle      int `json:"idle"`
	Completed int `json:"completed"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Conformance
// ──────────────────────────────────────────────────────────────────────────────

// ConformanceFeature is a capability that a harness may or may not support.
type ConformanceFeature string

// ConformanceTestResult records the outcome of a single feature test.
type ConformanceTestResult struct {
	Feature  ConformanceFeature `json:"feature"`
	Passed   bool               `json:"passed"`
	Skipped  bool               `json:"skipped,omitempty"`
	Error    string             `json:"error,omitempty"`
	Duration string             `json:"duration,omitempty"`
}

// ConformanceHarnessResult records all test results for a single harness.
type ConformanceHarnessResult struct {
	Harness   string                  `json:"harness"`
	Binary    string                  `json:"binary"`
	TestedAt  time.Time               `json:"tested_at"`
	Results   []ConformanceTestResult `json:"results"`
	Summary   ConformanceSummary      `json:"summary"`
}

// ConformanceSummary counts test outcomes.
type ConformanceSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// ConformanceMatrix holds conformance results for all tested harnesses.
type ConformanceMatrix struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Harnesses   []ConformanceHarnessResult `json:"harnesses"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Request types
// ──────────────────────────────────────────────────────────────────────────────

// CreateSessionRequest is the request body for POST /sessions.
type CreateSessionRequest struct {
	Harness       Harness         `json:"harness"`
	InstanceID    string          `json:"instance_id,omitempty"`
	DisplayName   string          `json:"display_name,omitempty"`
	AgentID       string          `json:"agent_id,omitempty"`
	SpawnerID     string          `json:"spawner_id,omitempty"`
	AutoStart     bool            `json:"auto_start,omitempty"`
	ClientID      string          `json:"client_id,omitempty"`
	HarnessConfig json.RawMessage `json:"harness_config,omitempty"` // opaque harness-specific config, merged into start params
}

// SendMessageRequest is the request body for POST /sessions/{id}/send.
type SendMessageRequest struct {
	Message string `json:"message"`
}

// ForkSessionRequest is the request body for POST /sessions/{id}/fork.
type ForkSessionRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
}

// CompactSessionRequest is the request body for POST /sessions/{id}/compact.
type CompactSessionRequest struct {
	Summary string `json:"summary,omitempty"`
}

// ConfigSessionRequest is the request body for POST /sessions/{id}/config.
type ConfigSessionRequest struct {
	Model         string   `json:"model,omitempty"`
	Effort        string   `json:"effort,omitempty"`
	DisabledTools []string `json:"disabled_tools,omitempty"`
	MaxBudget     *float64 `json:"max_budget,omitempty"`
}

// CreateInstanceRequest is the request body for POST /instances.
type CreateInstanceRequest struct {
	HarnessType           string    `json:"harness_type"`
	Name                  string    `json:"name"`
	Host                  string    `json:"host,omitempty"`
	Transport             Transport `json:"transport,omitempty"`
	SSHUser               string    `json:"ssh_user,omitempty"`
	SSHKeyPath            string    `json:"ssh_key_path,omitempty"`
	SSHPort               int       `json:"ssh_port,omitempty"`
	WorkingDir            string    `json:"working_dir,omitempty"`
	MaxConcurrentSessions int       `json:"max_concurrent_sessions,omitempty"`
}

// BindCredentialRequest is the request body for POST /instances/{id}/credentials.
type BindCredentialRequest struct {
	CredentialID string `json:"credential_id"`
	Priority     int    `json:"priority"`
}

// RenameSessionRequest is the request body for POST /sessions/{id}/rename.
type RenameSessionRequest struct {
	DisplayName string `json:"display_name"`
}
