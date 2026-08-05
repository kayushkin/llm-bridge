package msg

import "testing"

// The registry is a hand-maintained list, so the things that make it useful —
// unique names, real types, aliases that point somewhere — are exactly the
// things a careless edit breaks. Check them rather than trusting review.
func TestPurposeRegistryIsCoherent(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range KnownPurposes() {
		if p.Name == "" {
			t.Error("registry has an entry with no name")
			continue
		}
		if seen[p.Name] {
			t.Errorf("%s: duplicate entry", p.Name)
		}
		seen[p.Name] = true

		if !ValidSessionType(p.Type) {
			t.Errorf("%s: type %q is not a valid session type", p.Name, p.Type)
		}
		if p.Summary == "" {
			t.Errorf("%s: no summary — say what this kind of session does", p.Name)
		}
		if p.SupersededBy != "" {
			if _, ok := LookupPurpose(p.SupersededBy); !ok {
				t.Errorf("%s: superseded by %q, which is not in the registry", p.Name, p.SupersededBy)
			}
			if p.SupersededBy == p.Name {
				t.Errorf("%s: superseded by itself", p.Name)
			}
		}
	}
}

// An alias that files somewhere else than the purpose it aliases splits one
// folder into two, which is the drift the registry exists to prevent.
func TestSupersededPurposeFilesWithItsReplacement(t *testing.T) {
	for _, p := range KnownPurposes() {
		if p.SupersededBy == "" {
			continue
		}
		if got, want := FolderForPurpose(p.Name), FolderForPurpose(p.SupersededBy); got != want {
			t.Errorf("%s files into %q but %s files into %q", p.Name, got, p.SupersededBy, want)
		}
	}
}

func TestCanonicalPurpose(t *testing.T) {
	if got := CanonicalPurpose("kanban-dispatcher"); got != PurposeDispatcher {
		t.Errorf("CanonicalPurpose(kanban-dispatcher) = %q, want %q", got, PurposeDispatcher)
	}
	if got := CanonicalPurpose(PurposeAutoworker); got != PurposeAutoworker {
		t.Errorf("a current purpose must resolve to itself, got %q", got)
	}
	// An unknown purpose is returned unchanged rather than blanked, so a
	// caller that stores the result never loses the value it was handed.
	if got := CanonicalPurpose("nonsense"); got != "nonsense" {
		t.Errorf("CanonicalPurpose(nonsense) = %q, want it returned unchanged", got)
	}
}

func TestClassifyPurpose(t *testing.T) {
	cases := []struct {
		name      string
		typ       SessionType
		purpose   string
		origin    string
		wantKinds []string
	}{
		{
			name: "a coherent triple has nothing to report",
			typ:  SessionTypeAutonomous, purpose: PurposeAutoworker, origin: "scheduler",
			wantKinds: nil,
		},
		{
			name: "free text in purpose is the failure this catches",
			typ:  SessionTypeAutonomous, purpose: "dashv2 browser verification + A/B perf", origin: "bridge-agent",
			wantKinds: []string{"unknown-purpose"},
		},
		{
			name: "no purpose at all",
			typ:  SessionTypeInteractive, purpose: "", origin: "frontend",
			wantKinds: []string{"missing-purpose"},
		},
		{
			name: "a subagent claiming to be a human chat",
			typ:  SessionTypeInteractive, purpose: PurposeSubagent, origin: "llm-bridge-claudecode",
			wantKinds: []string{"type-mismatch"},
		},
		{
			name: "an old spelling still in use",
			typ:  SessionTypeAutonomous, purpose: "kanban-dispatcher", origin: "scheduler",
			wantKinds: []string{"superseded-purpose"},
		},
		{
			name: "a purpose showing up from somewhere new",
			typ:  SessionTypeAutonomous, purpose: PurposeAutoworker, origin: "somewhere-else",
			wantKinds: []string{"unexpected-origin"},
		},
		{
			name: "an empty origin is not reported here — create rejects it outright",
			typ:  SessionTypeAutonomous, purpose: PurposeAutoworker, origin: "",
			wantKinds: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyPurpose(tc.typ, tc.purpose, tc.origin)
			var kinds []string
			for _, p := range got {
				kinds = append(kinds, p.Kind)
			}
			if len(kinds) != len(tc.wantKinds) {
				t.Fatalf("got %v, want %v", kinds, tc.wantKinds)
			}
			for i := range kinds {
				if kinds[i] != tc.wantKinds[i] {
					t.Errorf("problem %d = %q, want %q", i, kinds[i], tc.wantKinds[i])
				}
			}
		})
	}
}

// Every type the registry references must be one the server will accept, or a
// caller doing exactly what the registry says gets a 400.
func TestRegisteredTypesAreAccepted(t *testing.T) {
	for _, p := range KnownPurposes() {
		if problems := ClassifyPurpose(p.Type, p.Name, ""); len(problems) > 0 {
			for _, pr := range problems {
				if pr.Kind != "superseded-purpose" {
					t.Errorf("%s: registry disagrees with itself: %s", p.Name, pr.Detail)
				}
			}
		}
	}
}

func TestValidSessionType(t *testing.T) {
	for _, ty := range SessionTypes() {
		if !ValidSessionType(ty) {
			t.Errorf("%q is listed by SessionTypes but rejected by ValidSessionType", ty)
		}
	}
	// Empty is a caller that failed to classify its session, not a default.
	if ValidSessionType("") {
		t.Error("empty session type must not be valid")
	}
	if ValidSessionType("chat") {
		t.Error("\"chat\" is a purpose, not a type, and must not validate")
	}
}
