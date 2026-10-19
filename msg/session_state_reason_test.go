package msg

import "testing"

// The vocabulary is one list: no reason twice, none without a summary, and
// ValidSessionStateReason reads exactly that list.
func TestSessionStateReasonsAreOneListOfDistinctDescribedValues(t *testing.T) {
	seen := map[SessionStateReason]bool{}
	for _, d := range SessionStateReasons() {
		if d.Reason == "" || d.Summary == "" {
			t.Errorf("entry %+v is missing its reason or its summary", d)
		}
		if seen[d.Reason] {
			t.Errorf("%s is listed twice", d.Reason)
		}
		seen[d.Reason] = true
		if !ValidSessionStateReason(d.Reason) {
			t.Errorf("%s is listed but not valid", d.Reason)
		}
	}
	if ValidSessionStateReason("tool=Bash") || ValidSessionStateReason("") {
		t.Error("a value outside the list reads as valid")
	}
}

func TestSessionStatusEqualComparesTheReason(t *testing.T) {
	a := SessionStatus{State: SessionCompleted, Reason: ReasonIdleReaped}
	b := SessionStatus{State: SessionCompleted, Reason: ReasonMarkDoneUser}
	if a.Equal(b) {
		t.Error("two statuses with different reasons read as equal; the second would never be sent")
	}
	c := SessionStatus{State: SessionError, Reason: ReasonSpawnFailed, ReasonDetail: "one cause"}
	d := SessionStatus{State: SessionError, Reason: ReasonSpawnFailed, ReasonDetail: "another cause"}
	if c.Equal(d) {
		t.Error("two statuses with different reason details read as equal")
	}
}
