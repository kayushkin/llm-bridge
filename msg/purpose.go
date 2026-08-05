package msg

import "sort"

// Session purpose registry.
//
// Purpose answers "what is this session for". It is a short slug from a
// known list, not a description — a session doing browser verification has
// purpose "delegate", and what it verified belongs in the display name.
// Free text here is the failure this registry exists to catch: it fills the
// column with one-off strings nobody can group, filter, or file.
//
// Each entry pins the type a session with that purpose must have, the
// origins expected to create it, and the sidebar folder it files into. That
// makes three questions answerable in code rather than by reading the table:
// is this purpose known, does its type agree with it, and where does it go.
//
// Adding a purpose means adding an entry here. That is the whole point: the
// list is short, reviewed, and in one file, so a new slug is a deliberate
// act rather than a typo that survives because nothing checked it.

// PurposeSpec describes one known session purpose.
type PurposeSpec struct {
	// Name is the slug stored in the session's purpose column.
	Name string

	// Type is the session type this purpose implies. A create request whose
	// declared type disagrees is a mistake by one side or the other, and the
	// server reports the disagreement rather than picking a winner.
	Type SessionType

	// Origins lists the services expected to create sessions with this
	// purpose. Empty means any origin is plausible. This is advisory — an
	// unexpected origin is worth surfacing, not worth a 400, because a new
	// caller adopting an existing purpose is normal and good.
	Origins []string

	// Folder is the sidebar folder a new session files into, or empty to
	// leave it unfiled. Replaces the LLMBRIDGE_SOURCE_FOLDERS env map, which
	// held the same data in a second place and drifted from it.
	Folder string

	// Summary is one line on what this kind of session does.
	Summary string

	// SupersededBy names the canonical purpose when this slug is a spelling
	// that got used before the list existed. Empty for current purposes.
	// Sessions keep the slug they were created with; the server resolves
	// through this field when it needs the canonical one.
	SupersededBy string
}

// purposeSpecs is the registry. Keep it sorted by name.
var purposeSpecs = []PurposeSpec{
	{
		Name:    PurposeAutoworker,
		Type:    SessionTypeAutonomous,
		Origins: []string{"scheduler"},
		Folder:  "Scheduled",
		Summary: "Nightly todo-worker run against the noteboard backlog.",
	},
	{
		Name:    PurposeChat,
		Type:    SessionTypeInteractive,
		Origins: []string{"frontend", "frontend-dash", "frontend-llmux", "llm-bridge-tui"},
		Summary: "A human talking to a harness through a frontend.",
	},
	{
		Name:    PurposeClassifier,
		Type:    SessionTypeSystem,
		Origins: []string{"scheduler"},
		Folder:  "Scheduled",
		Summary: "Kanban classifier turning live sessions into board cards.",
	},
	{
		Name: PurposeConformance,
		Type: SessionTypeSystem,
		// The bridge runs these, but a probe that leaked into the harness's
		// own history is re-found by discovery and attributed to the adapter
		// that found it. Both are true origins for a conformance session.
		Origins: []string{"llm-bridge-server", "llm-bridge-claudecode", "llm-bridge-codex"},
		Folder:  "Conformance",
		Summary: "Harness conformance probe run by the bridge itself.",
	},
	{
		Name:    PurposeDelegate,
		Type:    SessionTypeAutonomous,
		Origins: []string{"bridge-agent"},
		Summary: "One-shot run launched by the bridge-agent CLI.",
	},
	{
		Name:    PurposeDiscovered,
		Type:    SessionTypeExternal,
		Origins: []string{OriginDiscovery},
		Folder:  "Discovered",
		Summary: "Ran outside the bridge; imported from the harness's on-disk history.",
	},
	{
		Name:    PurposeDispatcher,
		Type:    SessionTypeAutonomous,
		Origins: []string{"scheduler"},
		Folder:  "Scheduled",
		Summary: "Kanban dispatcher reviving or starting work for a card.",
	},
	{
		Name:    PurposeE2E,
		Type:    SessionTypeSystem,
		Origins: []string{"e2e-smoke", "e2e-claude", "e2e-subagent"},
		Folder:  "Conformance",
		Summary: "End-to-end test script exercising the bridge.",
	},
	{
		Name:    PurposeExtraction,
		Type:    SessionTypeAutonomous,
		Origins: []string{"marginalia"},
		Summary: "Marginalia pulling facts and relations out of a document.",
	},
	{
		Name:    PurposeHarnessWatch,
		Type:    SessionTypeAutonomous,
		Origins: []string{"inber", "harness-watch.sh"},
		Folder:  "Scheduled",
		Summary: "Scheduled check that each harness still answers.",
	},
	{
		Name:    PurposeHealthcheck,
		Type:    SessionTypeAutonomous,
		Origins: []string{"healthcheck"},
		Folder:  "Scheduled",
		Summary: "Agent spawned to act on a failing resource or service alert.",
	},
	{
		Name:    PurposeHerald,
		Type:    SessionTypeHerald,
		Origins: []string{"ask"},
		Summary: "Question or alert an agent is relaying to the user.",
	},
	{
		Name:         "kanban-dispatcher",
		Type:         SessionTypeAutonomous,
		Origins:      []string{"scheduler"},
		Folder:       "Scheduled",
		Summary:      "Older spelling of dispatcher.",
		SupersededBy: PurposeDispatcher,
	},
	{
		Name:    PurposeRenamer,
		Type:    SessionTypeSystem,
		Origins: []string{"llm-bridge-server"},
		Folder:  "Auto-rename",
		Summary: "Meta-agent that names a session from its first turn.",
	},
	{
		Name:         "scheduler",
		Type:         SessionTypeAutonomous,
		Origins:      []string{"scheduler"},
		Folder:       "Scheduled",
		Summary:      "Older spelling of scheduled-task.",
		SupersededBy: PurposeScheduledTask,
	},
	{
		Name:    PurposeScheduledTask,
		Type:    SessionTypeAutonomous,
		Origins: []string{"scheduler"},
		Folder:  "Scheduled",
		Summary: "Agent-type cron job fired by the scheduler.",
	},
	{
		Name:    PurposeScoper,
		Type:    SessionTypeSystem,
		Origins: []string{"scheduler"},
		Folder:  "Scheduled",
		Summary: "Decomposes a goal card into sub-cards.",
	},
	{
		Name:    PurposeSubagent,
		Type:    SessionTypeSystem,
		Origins: []string{"llm-bridge-claudecode", "llm-bridge-codex"},
		Folder:  "Subagents",
		Summary: "Subagent spawned by a parent harness through its Task tool.",
	},
	{
		Name:    PurposeWorkflowSubagent,
		Type:    SessionTypeSystem,
		Origins: []string{"llm-bridge-claudecode"},
		Folder:  "Subagents",
		Summary: "Subagent spawned by a parent harness through its Workflow tool.",
	},
}

// Known purpose slugs. Use these constants rather than string literals so a
// rename is a compile error at every call site instead of a silent miss.
const (
	PurposeAutoworker       = "autoworker"
	PurposeChat             = "chat"
	PurposeClassifier       = "classifier"
	PurposeConformance      = "conformance"
	PurposeDelegate         = "delegate"
	PurposeDiscovered       = "discovered"
	PurposeDispatcher       = "dispatcher"
	PurposeE2E              = "e2e"
	PurposeExtraction       = "extraction"
	PurposeHarnessWatch     = "harness-watch"
	PurposeHealthcheck      = "healthcheck"
	PurposeHerald           = "herald"
	PurposeRenamer          = "renamer"
	PurposeScheduledTask    = "scheduled-task"
	PurposeScoper           = "scoper"
	PurposeSubagent         = "subagent"
	PurposeWorkflowSubagent = "workflow-subagent"
)

// OriginDiscovery is the origin written for sessions the bridge imported from
// a harness's on-disk history rather than created. It is deliberately not the
// adapter's name: the adapter found the session, and origin means who made it.
const OriginDiscovery = "discovery"

var purposeByName = func() map[string]PurposeSpec {
	m := make(map[string]PurposeSpec, len(purposeSpecs))
	for _, p := range purposeSpecs {
		m[p.Name] = p
	}
	return m
}()

// LookupPurpose returns the spec for a purpose slug. The second result is
// false for anything not in the registry, which is the caller's signal to
// flag the value rather than to reject it — see ClassifyPurpose.
func LookupPurpose(name string) (PurposeSpec, bool) {
	p, ok := purposeByName[name]
	return p, ok
}

// CanonicalPurpose resolves a superseded slug to its current spelling and
// returns anything else unchanged, including unknown values.
func CanonicalPurpose(name string) string {
	if p, ok := purposeByName[name]; ok && p.SupersededBy != "" {
		return p.SupersededBy
	}
	return name
}

// KnownPurposes returns every registered spec, sorted by name. Superseded
// entries are included; filter on SupersededBy to list only current ones.
func KnownPurposes() []PurposeSpec {
	out := make([]PurposeSpec, len(purposeSpecs))
	copy(out, purposeSpecs)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FolderForPurpose returns the sidebar folder a purpose files into, or empty
// for none. Unknown purposes file nowhere, which leaves them visible in the
// unfiled list rather than tucked into a folder under a name nobody chose.
func FolderForPurpose(name string) string {
	if p, ok := purposeByName[name]; ok {
		return p.Folder
	}
	return ""
}

// PurposeProblem is one disagreement between a session's declared
// classification and the registry.
type PurposeProblem struct {
	// Kind is one of: "unknown-purpose", "superseded-purpose",
	// "type-mismatch", "unexpected-origin", "missing-purpose".
	Kind string `json:"kind"`
	// Detail explains the disagreement in one sentence.
	Detail string `json:"detail"`
	// Want is the value the registry expects, where there is one.
	Want string `json:"want,omitempty"`
	// Got is the value that arrived.
	Got string `json:"got,omitempty"`
}

// ClassifyPurpose checks a (type, purpose, origin) triple against the
// registry and returns every disagreement it finds. An empty result means
// the triple is coherent.
//
// This reports rather than decides. None of these problems is grounds for
// refusing a session on its own: a new caller adopting an existing purpose,
// or an existing caller adopting a new one, both look like problems here and
// both are fine. The server rejects only what it cannot store honestly — an
// unknown type or a missing origin — and leaves the rest to the guard that
// reads these problems on a schedule.
func ClassifyPurpose(t SessionType, purpose, origin string) []PurposeProblem {
	var out []PurposeProblem

	if purpose == "" {
		return append(out, PurposeProblem{
			Kind:   "missing-purpose",
			Detail: "session declares no purpose, so it cannot be grouped or filed",
		})
	}

	spec, ok := LookupPurpose(purpose)
	if !ok {
		return append(out, PurposeProblem{
			Kind:   "unknown-purpose",
			Detail: "purpose is not in the registry; add it to msg/purpose.go or use an existing slug",
			Got:    purpose,
		})
	}

	if spec.SupersededBy != "" {
		out = append(out, PurposeProblem{
			Kind:   "superseded-purpose",
			Detail: "purpose has a current spelling",
			Want:   spec.SupersededBy,
			Got:    purpose,
		})
	}

	if t != spec.Type {
		out = append(out, PurposeProblem{
			Kind:   "type-mismatch",
			Detail: "declared type disagrees with the type this purpose implies",
			Want:   string(spec.Type),
			Got:    string(t),
		})
	}

	if len(spec.Origins) > 0 && origin != "" {
		known := false
		for _, o := range spec.Origins {
			if o == origin {
				known = true
				break
			}
		}
		if !known {
			out = append(out, PurposeProblem{
				Kind:   "unexpected-origin",
				Detail: "origin is not one the registry associates with this purpose",
				Got:    origin,
			})
		}
	}

	return out
}
