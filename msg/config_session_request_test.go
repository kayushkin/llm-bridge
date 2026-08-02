package msg

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestConfigSessionRequestCarriesAnEmptyDisabledToolsList pins the one value
// this struct exists to carry across the wire.
//
// DisabledTools is the whole set, not an addition to it, so a caller sends an
// empty list to re-enable every tool. Absent means "leave the tool set alone".
// Both receivers of "config:<json>" — llm-bridge-jig's handleConfig and inber's
// handleBridgeConfig — test the decoded field for nil rather than for length,
// precisely so they can tell those two apart.
//
// bridge-server does not forward the caller's bytes: handleConfigSession
// decodes into this struct and re-marshals it. So whether the distinction
// survives at all is decided here, by this struct's tags, and nowhere else.
func TestConfigSessionRequestCarriesAnEmptyDisabledToolsList(t *testing.T) {
	req := ConfigSessionRequest{DisabledTools: []string{}}

	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"disabled_tools"`) {
		t.Fatalf("an explicitly empty disabled_tools list was dropped on the way out: %s\n"+
			"the receiver reads an absent field as \"leave the tool set alone\", so the "+
			"re-enable-everything request arrives as a no-op", encoded)
	}

	var round ConfigSessionRequest
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.DisabledTools == nil {
		t.Fatalf("round trip turned an explicit empty list into an absent one: %s", encoded)
	}
	if len(round.DisabledTools) != 0 {
		t.Fatalf("round trip invented entries: %#v", round.DisabledTools)
	}
}

// TestConfigSessionRequestKeepsAnAbsentDisabledToolsListAbsent is the
// complement, and it is the half that makes the fix safe rather than merely
// louder: a request that says nothing about tools must keep saying nothing.
// JSON null decodes into a nil slice, which is the same answer an omitted key
// gives, so both receivers still read "leave the tool set alone".
func TestConfigSessionRequestKeepsAnAbsentDisabledToolsListAbsent(t *testing.T) {
	encoded, err := json.Marshal(ConfigSessionRequest{Model: "sonnet"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round ConfigSessionRequest
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.DisabledTools != nil {
		t.Fatalf("a request that named no tool set came back naming one: %#v (%s)",
			round.DisabledTools, encoded)
	}
	if round.Model != "sonnet" {
		t.Fatalf("model did not survive: %q", round.Model)
	}
}
