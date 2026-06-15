package msg

import "testing"

func TestSessionStateIsBlockedOnUser(t *testing.T) {
	blocked := []SessionState{
		SessionAwaitingPermission,
		SessionAwaitingUser,
		SessionWaitingApproval, // deprecated alias for awaiting_permission
	}
	for _, s := range blocked {
		if !s.IsBlockedOnUser() {
			t.Errorf("IsBlockedOnUser(%q) = false, want true", s)
		}
	}

	notBlocked := []SessionState{
		SessionStarting,
		SessionModelGenerating,
		SessionToolRunning,
		SessionCompacting,
		SessionRateLimited,
		SessionRunning,
		SessionPaused,
		SessionIdle,
		SessionCompleted,
		SessionError,
		SessionAborted,
		SessionDisconnected,
	}
	for _, s := range notBlocked {
		if s.IsBlockedOnUser() {
			t.Errorf("IsBlockedOnUser(%q) = true, want false", s)
		}
	}
}

// A state must never be both active and blocked-on-user: the reaper relies on
// the two predicates partitioning the non-reapable states without overlap, and
// IsActive's own contract is "live or expected-to-be-live work".
func TestSessionStateActiveAndBlockedAreDisjoint(t *testing.T) {
	all := []SessionState{
		SessionStarting, SessionModelGenerating, SessionToolRunning,
		SessionCompacting, SessionAwaitingPermission, SessionAwaitingUser,
		SessionRateLimited, SessionPaused, SessionIdle, SessionCompleted,
		SessionError, SessionAborted, SessionDisconnected, SessionRunning,
		SessionWaitingApproval,
	}
	for _, s := range all {
		if s.IsActive() && s.IsBlockedOnUser() {
			t.Errorf("state %q is both IsActive and IsBlockedOnUser", s)
		}
	}
}
