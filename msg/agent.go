package msg

import "time"

// AgentDef is the canonical agent shape — harness-agnostic. Sourced from
// agent-store. Renderers consume this via AgentView; harness bridges
// consume this via AgentReconciler.EnsureAgent. See
// ~/repos/llm-bridge-server/AGENT-MANAGEMENT.md.
type AgentDef struct {
	ID              int64     `json:"id"`
	Slug            string    `json:"slug"`
	DisplayName     string    `json:"display_name"`
	Emoji           string    `json:"emoji,omitempty"`
	Description     string    `json:"description,omitempty"`
	Role            string    `json:"role,omitempty"`
	Enabled         bool      `json:"enabled"`
	ParentAgentID   *int64    `json:"parent_agent_id,omitempty"`
	Identity        string    `json:"identity,omitempty"`      // assembled from agent_nature
	HarnessConfig   *HarnessAgentConfig `json:"harness_config,omitempty"` // per-harness row (model, budgets, system_prompt override)
	Tools           []ToolEnrollment    `json:"tools,omitempty"`
	Skills          []SkillEnrollment   `json:"skills,omitempty"`
	Subagents       []AgentDef          `json:"subagents,omitempty"` // recursive; populated for parent agents
	Extras          []NamedSection      `json:"extras,omitempty"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// HarnessAgentConfig is the per-(agent, harness) row from agent-store.
// agent_harness table. Optional fields default to zero if unset.
type HarnessAgentConfig struct {
	HarnessID         string   `json:"harness_id"`
	HarnessAgentID    string   `json:"harness_agent_id"`
	SystemPrompt      string   `json:"system_prompt,omitempty"`
	ModelPrimary      string   `json:"model_primary,omitempty"`
	ModelFallbacks    []string `json:"model_fallbacks,omitempty"`
	WorkspacePath     string   `json:"workspace_path,omitempty"`
	ThinkingBudget    int      `json:"thinking_budget,omitempty"`
	ContextBudget     int      `json:"context_budget,omitempty"`
	ContextTags       []string `json:"context_tags,omitempty"`
	MaxTurns          int      `json:"max_turns,omitempty"`
	MaxInputTokens    int      `json:"max_input_tokens,omitempty"`
	MaxResponseTime   int      `json:"max_response_time,omitempty"`
	SubagentAllow     []string `json:"subagent_allow,omitempty"`
	Enabled           bool     `json:"enabled"`
	Shelved           bool     `json:"shelved,omitempty"`
	IsDefault         bool     `json:"is_default,omitempty"`
}

// ToolEnrollment is one tool an agent has access to under a given harness.
// Maps to a row in agent-store's agent_harness_tools.
type ToolEnrollment struct {
	HarnessID string `json:"harness_id"`
	ToolName  string `json:"tool_name"`
}

// SkillEnrollment is one skill an agent has access to. Skill enrollment
// is harness-agnostic at the canonical level; per-harness rendering
// decides whether to inject as full body or header-only.
type SkillEnrollment struct {
	SkillID string `json:"skill_id"`
}

// NamedSection is a freeform per-agent instruction block (e.g. extras
// outside of identity). Concatenated into the rendered prompt in
// alphabetical Name order for deterministic cache behavior.
type NamedSection struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// AgentView is the read-only projection passed to a Renderer. The
// renderer is a pure function — same AgentView in, same bytes out.
type AgentView struct {
	Agent     AgentDef
	Harness   Harness // which harness we're rendering for
}
