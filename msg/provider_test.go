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
		SessionWaitingApproval, SessionBackgroundTasksRunning,
	}
	for _, s := range all {
		if s.IsActive() && s.IsBlockedOnUser() {
			t.Errorf("state %q is both IsActive and IsBlockedOnUser", s)
		}
	}
}

// A turn that ended with subagents or backgrounded commands still running is
// live work. Reported as idle it was invisible to the restart reconcile, which
// selects on ActiveSessionStates, and a redeploy killed it for good.
func TestBackgroundTasksRunningIsActiveAndNotBlockedOnUser(t *testing.T) {
	if !SessionBackgroundTasksRunning.IsActive() {
		t.Error("background_tasks_running must be IsActive, or a restart will not resume it")
	}
	if SessionBackgroundTasksRunning.IsBlockedOnUser() {
		t.Error("background_tasks_running is not waiting on a person")
	}
}

// ActiveSessionStates is the SQL-side copy of IsActive. They are two lists of
// the same fact, so a state added to one and not the other is resumed by the
// watchdog and skipped by the reconcile, or the reverse.
func TestActiveSessionStatesAgreesWithIsActive(t *testing.T) {
	listed := map[SessionState]bool{}
	for _, s := range ActiveSessionStates() {
		listed[s] = true
		if !s.IsActive() {
			t.Errorf("ActiveSessionStates lists %q but IsActive says false", s)
		}
	}
	for _, s := range []SessionState{
		SessionStarting, SessionModelGenerating, SessionToolRunning,
		SessionCompacting, SessionBackgroundTasksRunning, SessionAwaitingPermission,
		SessionAwaitingUser, SessionRateLimited, SessionPaused, SessionIdle,
		SessionCompleted, SessionError, SessionAborted, SessionDisconnected,
		SessionRunning, SessionWaitingApproval,
	} {
		if s.IsActive() && !listed[s] {
			t.Errorf("%q is IsActive but missing from ActiveSessionStates", s)
		}
	}
}
