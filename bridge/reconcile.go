package bridge

import (
	"context"

	"github.com/kayushkin/llm-bridge/msg"
)

// AgentReconciler is the per-harness adapter that reconciles agent-store
// definitions with whatever filesystem / config state the harness expects,
// and produces a SpawnSpec for starting a session. Distinct from
// HarnessBridge (which manages session lifecycle) — both interfaces are
// typically implemented by the same llm-bridge-<harness> wrapper.
//
// See ~/repos/llm-bridge-server/HARNESS-LAYER.md for the full contract.
type AgentReconciler interface {
	// Harness returns which harness this reconciler handles.
	Harness() msg.Harness

	// EnsureAgent reconciles the agent's static state with what the harness
	// expects. Idempotent. Called when an agent is created or its definition
	// changes — NOT on every session.
	//
	// For Claude Code: writes per-agent MCP config files (when the agent has
	// MCP-typed tool-store entries) and per-agent settings.json. Does NOT
	// materialize subagent files (subagents are passed via --agents JSON at
	// session spawn — verified 2026-05-09).
	//
	// For harnesses without per-agent files (inber, hermes), this is a no-op.
	EnsureAgent(ctx context.Context, agent msg.AgentDef) error

	// PrepareSession resolves the per-spawn details needed to start a session
	// for this agent. Does not write per-session files; pure data result.
	// Called on every session.create.
	PrepareSession(ctx context.Context, sess SessionRef) (SpawnSpec, error)

	// CleanupAgent removes any harness-side artifacts for an agent being
	// deleted. Idempotent.
	CleanupAgent(ctx context.Context, agentID int64) error
}

// SessionRef identifies which session is being prepared and which agent it
// runs. The reconciler reads agent definition fresh from agent-store at this
// point, in case the agent was edited since the last EnsureAgent.
type SessionRef struct {
	SessionID string
	AgentID   int64
	CWD       string // requested CWD (typically the user's repo); reconciler may pass through or override
}

// SpawnSpec is what bridge-server uses to actually fork the harness wrapper
// subprocess. Pure data — no filesystem side effects implied. The reconciler
// fills this in based on the agent's current state.
type SpawnSpec struct {
	Args       []string          // additional CLI args to pass to the harness wrapper
	Env        map[string]string // additional environment variables
	CWD        string             // working directory (usually the user's repo, not a manufactured workspace)
	BundleHash string            // SHA256 of the composed identity bundle, for cache observability
}
