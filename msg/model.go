package msg

// Model identity and selection types.
//
// model-store is the ONE source of truth for what models exist and what they
// cost. Nothing here defines a model; these types carry a model's identity
// across the wire and record which layer SELECTED it for a given call. There is
// deliberately no notion of a second "source" — the layers below the registry
// only ever select or override, never originate, a model.

// ModelID is a model's registry id — model-store's models.id, e.g.
// "claude-fable-5-1". It is a named type so a model id cannot be passed where
// an arbitrary string is expected, and an arbitrary string cannot silently
// stand in for a model id. Carry this, not a bare string, wherever a model is
// named.
type ModelID string

// ModelRole is a purpose-named pointer into the registry, resolved by
// model-store to a concrete ModelID. Roles are global, not per-provider: a role
// names a capability tier that any harness can be pointed at, which is why a
// caller asks for "the efficient model" rather than pinning an id it must then
// keep current. This set mirrors model-store's canonical roles exactly.
type ModelRole string

const (
	// ModelRoleBest is the highest-performance model, cost no object.
	ModelRoleBest ModelRole = "best"
	// ModelRoleDefault is the everyday model: much cheaper than best, better
	// than efficient. It is the tier used when no other layer overrides.
	ModelRoleDefault ModelRole = "default"
	// ModelRoleEfficient is cheap enough to run at volume — titles,
	// classification, polling and other high-count one-shot calls.
	ModelRoleEfficient ModelRole = "efficient"
)

// CanonicalModelRoles is the single definition of the valid role set, in the
// order a UI should surface them. It mirrors model-store's CanonicalRoles;
// nothing else may carry its own copy.
var CanonicalModelRoles = []ModelRole{ModelRoleBest, ModelRoleDefault, ModelRoleEfficient}

// Model mirrors a model-store registry row — the single source of truth. This
// is the shared wire/TS shape; model-store's own Model is the same shape and is
// expected to converge on this type rather than keep a second, drifting copy.
// A model's context window is MaxTokens: there is no separate context-variant
// concept (no "[1m]" tag), because a variant with a different window is either
// its own row or the same row's declared MaxTokens.
type Model struct {
	ID         ModelID  `json:"id"`
	Provider   string   `json:"provider"`
	Name       string   `json:"name"`
	ShortName  string   `json:"short_name"`
	Aliases    []string `json:"aliases"`
	MaxTokens  int      `json:"max_tokens"`
	InputCost  float64  `json:"input_cost"`
	OutputCost float64  `json:"output_cost"`
	Enabled    bool     `json:"enabled"`
	Priority   int      `json:"priority"`
}

// ModelSelectedBy names which layer's choice decided a call's model. It is NOT
// a model source — model-store is the only source. It records the override
// layer that won, so a resolved selection can be explained and reconciled
// against what actually ran.
type ModelSelectedBy string

const (
	// ModelSelectedBySession: a per-session / per-turn override chose the model.
	ModelSelectedBySession ModelSelectedBy = "session"
	// ModelSelectedByPrefs: no session override, so the per-harness default in
	// bridge-prefs chose it.
	ModelSelectedByPrefs ModelSelectedBy = "prefs"
	// ModelSelectedByRole: nothing overrode, so resolution fell through to a
	// model-store role (normally ModelRoleDefault). This is the floor: if even
	// the role is unassigned, resolution fails loud rather than inventing a
	// harness default, so an implicit account-tier default can never win
	// silently.
	ModelSelectedByRole ModelSelectedBy = "role"
)

// ModelSelection is the outcome of resolving which model a call should use. It
// carries the resolved id, the role that was asked for (if the request named a
// role rather than an id), and the layer that decided. Reconciliation compares
// Model against what actually ran per query source.
type ModelSelection struct {
	Model      ModelID         `json:"model"`
	Role       ModelRole       `json:"role,omitempty"`
	SelectedBy ModelSelectedBy `json:"selected_by"`
}
