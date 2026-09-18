package msg

// EffectiveConfig is what a session is given, setting by setting, with the
// layer that decided each — the answer to "why is this session on this model,
// as this principal, with these tools". llm-bridge-server serves it for a
// stored session (GET /sessions/{id}/effective-config) and as a dry run for a
// session that does not exist yet (GET /effective-config?harness=…).
//
// It reports; it decides nothing. Every value comes from the same code the
// spawn uses, so the view and the spawn cannot disagree.
type EffectiveConfig struct {
	Subject  EffectiveConfigSubject `json:"subject"`
	Settings []EffectiveSetting     `json:"settings"`
	// Warnings are things the reader should know that no one setting owns:
	// a store that could not be asked, a grant check that would refuse the
	// spawn, a row that predates a snapshot.
	Warnings []string `json:"warnings"`
	// Layers is the served vocabulary EffectiveSetting.Layer draws from, in
	// precedence order, so a UI draws a legend from the answer rather than
	// from a copy.
	Layers []EffectiveConfigLayerDefinition `json:"layers"`
}

// EffectiveConfigSubject is what the settings were computed for: a stored
// session, or the inputs of a dry run.
type EffectiveConfigSubject struct {
	SessionID   string   `json:"session_id,omitempty"`
	DryRun      bool     `json:"dry_run"`
	Harness     Harness  `json:"harness"`
	InstanceID  string   `json:"instance_id,omitempty"`
	PrincipalID string   `json:"principal_id,omitempty"`
	AgentID     string   `json:"agent_id,omitempty"`
	BundleID    string   `json:"bundle_id,omitempty"`
	BoardID     string   `json:"board_id,omitempty"`
	CardID      string   `json:"card_id,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// EffectiveSetting is one decided value.
type EffectiveSetting struct {
	Key EffectiveSettingKey `json:"key"`
	// Value is the decided value: a string, a number, a list, or null when
	// nothing decides it (Layer is then EffectiveLayerNone).
	Value any                  `json:"value" tstype:"unknown"`
	Layer EffectiveConfigLayer `json:"layer"`
	// Record names where the value is stored, in the owning service's terms,
	// e.g. "bridge-prefs.defaults.claude_code.model" or "kanban-store board
	// 0a25…/tag rule tr_3".
	Record string `json:"record,omitempty"`
	// Detail is one sentence on how this layer came to decide it.
	Detail string `json:"detail,omitempty"`
	// Notes are the layers that were consulted and did not decide, or that
	// narrowed the value: "bundle names 13 14; grants allow 14".
	Notes []string `json:"notes,omitempty"`
}

// EffectiveSettingKey names the settings the view reports, one per row.
type EffectiveSettingKey string

const (
	EffectiveSettingModel          EffectiveSettingKey = "model"
	EffectiveSettingEffort         EffectiveSettingKey = "effort"
	EffectiveSettingMaxBudget      EffectiveSettingKey = "max_budget"
	EffectiveSettingDisabledTools  EffectiveSettingKey = "disabled_tools"
	EffectiveSettingPermissionMode EffectiveSettingKey = "permission_mode"
	EffectiveSettingPrincipal      EffectiveSettingKey = "principal"
	EffectiveSettingInstance       EffectiveSettingKey = "instance"
	EffectiveSettingAgent          EffectiveSettingKey = "agent"
	EffectiveSettingBundle         EffectiveSettingKey = "bundle"
	EffectiveSettingTools          EffectiveSettingKey = "tools"
)

// EffectiveSettingKeys is every key, in the order a view should list them.
var EffectiveSettingKeys = []EffectiveSettingKey{
	EffectiveSettingModel, EffectiveSettingEffort, EffectiveSettingMaxBudget, EffectiveSettingDisabledTools,
	EffectiveSettingPermissionMode, EffectiveSettingPrincipal, EffectiveSettingInstance, EffectiveSettingAgent,
	EffectiveSettingBundle, EffectiveSettingTools,
}

// EffectiveConfigLayer names which layer decided a setting.
type EffectiveConfigLayer string

const (
	// EffectiveLayerSession: the session's own row or harness_config — set at
	// create by the caller, by a later POST /sessions/{id}/config, or pinned
	// there by the server at create (model, permission mode).
	EffectiveLayerSession EffectiveConfigLayer = "session"
	// EffectiveLayerRequest: named by whoever created the session and stored
	// as given — the instance, the principal, the agent, the bundle.
	EffectiveLayerRequest EffectiveConfigLayer = "request"
	// EffectiveLayerTagRule: a kanban board's tag rule matched the card.
	EffectiveLayerTagRule EffectiveConfigLayer = "tag_rule"
	// EffectiveLayerBoard: a kanban board's own default.
	EffectiveLayerBoard EffectiveConfigLayer = "board"
	// EffectiveLayerBundle: the bundle the session was started with.
	EffectiveLayerBundle EffectiveConfigLayer = "bundle"
	// EffectiveLayerGrant: the principal's grants (grant-store) narrowed or
	// allowed the value.
	EffectiveLayerGrant EffectiveConfigLayer = "grant"
	// EffectiveLayerInstance: the instance's own opt-ins in tool-store.
	EffectiveLayerInstance EffectiveConfigLayer = "instance"
	// EffectiveLayerHarnessDefault: bridge-prefs.defaults[harness], the
	// Settings page's per-harness section.
	EffectiveLayerHarnessDefault EffectiveConfigLayer = "harness_default"
	// EffectiveLayerGlobal: a bridge-prefs field that applies everywhere —
	// permission_mode, default_principal_id.
	EffectiveLayerGlobal EffectiveConfigLayer = "global"
	// EffectiveLayerRegistry: model-store's default role, the floor under the
	// model.
	EffectiveLayerRegistry EffectiveConfigLayer = "registry"
	// EffectiveLayerNone: nothing decides it; the harness does what it does.
	EffectiveLayerNone EffectiveConfigLayer = "none"
)

// EffectiveConfigLayerDefinition is one row of the served legend.
type EffectiveConfigLayerDefinition struct {
	Layer       EffectiveConfigLayer `json:"layer"`
	Description string               `json:"description"`
}

// EffectiveConfigLayers is the legend, narrowest first: a layer listed
// earlier outranks one listed later wherever both can decide the same setting.
var EffectiveConfigLayers = []EffectiveConfigLayerDefinition{
	{EffectiveLayerSession, "The session's own row or harness_config: set at create, changed by POST /sessions/{id}/config or the chat header, or pinned by the server at create."},
	{EffectiveLayerRequest, "Named by whoever created the session, stored as given."},
	{EffectiveLayerTagRule, "A kanban board tag rule that matched the card's tags."},
	{EffectiveLayerBoard, "The kanban board's own default."},
	{EffectiveLayerBundle, "The bundle the session was started with, resolved by bundle-store."},
	{EffectiveLayerGrant, "The principal's grants in grant-store, narrowing what is offered or gating where it runs."},
	{EffectiveLayerInstance, "The instance's tool opt-ins in tool-store."},
	{EffectiveLayerHarnessDefault, "bridge-prefs.defaults for the harness — the Settings page's per-harness section."},
	{EffectiveLayerGlobal, "A bridge-prefs field that applies everywhere: permission_mode, default_principal_id."},
	{EffectiveLayerRegistry, "model-store's default role, the floor under the model."},
	{EffectiveLayerNone, "Nothing decides it; the harness does what it does."},
}
