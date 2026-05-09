package identity

import "testing"

// counter is a deterministic minter for tests: returns "msg_1", "msg_2", ...
type counter struct{ n int }

func (c *counter) mint() string {
	c.n++
	return mintN(c.n)
}

func mintN(n int) string {
	return "msg_" + itoa(n)
}

func itoa(n int) string {
	// minimal integer-to-string to avoid pulling strconv into a tiny test
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func newTestTracker() (*Tracker, *counter) {
	c := &counter{}
	return NewTracker(NewMemoryStore(), c.mint), c
}

func TestAssignMessageID_FirstEventMintsBubble(t *testing.T) {
	tr, _ := newTestTracker()
	got, err := tr.AssignMessageID("h1")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if got != "msg_1" {
		t.Errorf("got %q, want msg_1", got)
	}
}

func TestAssignMessageID_RepeatedHarnessIDReusesOpenBubble(t *testing.T) {
	tr, _ := newTestTracker()
	first, _ := tr.AssignMessageID("h1")
	second, _ := tr.AssignMessageID("h1")
	if first != second {
		t.Errorf("repeated h1 should reuse bubble: %q vs %q", first, second)
	}
}

func TestAssignMessageID_EmptyHarnessIDReusesOpenBubble(t *testing.T) {
	tr, _ := newTestTracker()
	first, _ := tr.AssignMessageID("h1")
	second, _ := tr.AssignMessageID("") // stream delta with no parent harness id
	if first != second {
		t.Errorf("empty hid should reuse open bubble: %q vs %q", first, second)
	}
}

func TestAssignMessageID_NewHarnessIDSplitsBubble(t *testing.T) {
	tr, _ := newTestTracker()
	first, _ := tr.AssignMessageID("h1")
	second, _ := tr.AssignMessageID("h2")
	if first == second {
		t.Errorf("new harness id should split bubble: both %q", first)
	}
	if first != "msg_1" || second != "msg_2" {
		t.Errorf("got %q + %q, want msg_1 + msg_2", first, second)
	}
}

func TestAssignMessageID_ResumeReusesPriorBinding(t *testing.T) {
	tr, _ := newTestTracker()
	first, _ := tr.AssignMessageID("h1")
	tr.EndTurn() // simulate result event closing the turn
	// Adapter restarts mid-stream; the harness re-emits an event for h1.
	second, _ := tr.AssignMessageID("h1")
	if first != second {
		t.Errorf("resume should reuse prior binding for h1: %q vs %q", first, second)
	}
}

func TestEndTurn_ClearsOpenBubble(t *testing.T) {
	tr, _ := newTestTracker()
	tr.AssignMessageID("h1")
	if tr.OpenMessageID() == "" {
		t.Fatal("expected open bubble before EndTurn")
	}
	tr.EndTurn()
	if tr.OpenMessageID() != "" {
		t.Errorf("EndTurn should clear open bubble; got %q", tr.OpenMessageID())
	}
	// Next assign with no harness id mints a fresh bubble (no prior binding to reuse).
	next, _ := tr.AssignMessageID("")
	if next != "msg_2" {
		t.Errorf("post-EndTurn fresh bubble should be msg_2; got %q", next)
	}
}

func TestAssignMessageID_BindingPersistsAcrossSplits(t *testing.T) {
	tr, _ := newTestTracker()
	first, _ := tr.AssignMessageID("h1")
	tr.AssignMessageID("h2") // split: open new bubble
	// Harness re-emits h1 (rare but possible mid-turn re-broadcast).
	resumed, _ := tr.AssignMessageID("h1")
	if resumed != first {
		t.Errorf("h1 should resolve back to its original bubble %q; got %q", first, resumed)
	}
}
