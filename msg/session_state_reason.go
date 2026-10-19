package msg

// SessionStateReason says why a session's state last changed. It travels on
// SessionStatus.Reason and on StateEvent.Reason, and llm-bridge-server sets it
// from this list and nothing else: a reason not declared here is a bug in the
// server, not a new word for clients to learn.
//
// It exists because a state alone cannot say how a session got there. Until
// 2026-10 an autoworker that finished its work and was then killed by the idle
// reaper read `aborted`, exactly like one a person stopped, and the forced
// status carried no reason at all; the only trace was a journal line.
type SessionStateReason string

// Reasons a derived transition carries: the harness's own events implied it.
const (
	ReasonTurnStarted                        SessionStateReason = "turn_started"
	ReasonToolCalled                         SessionStateReason = "tool_called"
	ReasonToolsDrained                       SessionStateReason = "tools_drained"
	ReasonCompactRequested                   SessionStateReason = "compact_requested"
	ReasonHarnessCompacting                  SessionStateReason = "harness_compacting"
	ReasonCompactComplete                    SessionStateReason = "compact_complete"
	ReasonBackgroundTasksRunning             SessionStateReason = "background_tasks_running"
	ReasonBackgroundTasksFinished            SessionStateReason = "background_tasks_finished"
	ReasonPermissionRequested                SessionStateReason = "permission_requested"
	ReasonPermissionResolved                 SessionStateReason = "permission_resolved"
	ReasonApprovalRequired                   SessionStateReason = "approval_required"
	ReasonApprovalResolved                   SessionStateReason = "approval_resolved"
	ReasonTurnComplete                       SessionStateReason = "turn_complete"
	ReasonTurnCompleteAwaitingUser           SessionStateReason = "turn_complete_awaiting_user"
	ReasonTurnCompleteBackgroundTasksRunning SessionStateReason = "turn_complete_background_tasks_running"
	ReasonHarnessError                       SessionStateReason = "error"
	ReasonAborted                            SessionStateReason = "aborted"
	ReasonLifecycle                          SessionStateReason = "lifecycle"
	ReasonTurnSettledReconcile               SessionStateReason = "turn_settled_reconcile"
)

// Reasons for a state decided after the turn's own events, by the turn-end
// classifier or by a person acting on a signal.
const (
	ReasonTurnCompleteSignalQuestion     SessionStateReason = "turn_complete_signal_question"
	ReasonTurnCompleteSignalNotification SessionStateReason = "turn_complete_signal_notification"
	ReasonTurnCompleteSignalNeither      SessionStateReason = "turn_complete_signal_neither"
	ReasonSignalDismissed                SessionStateReason = "signal_dismissed"
	ReasonMarkDoneUser                   SessionStateReason = "mark_done_user"
)

// Reasons for a state the server decided on its own: no harness event says it.
const (
	ReasonProcessSpawned      SessionStateReason = "process_spawned"
	ReasonSpawnFailed         SessionStateReason = "spawn_failed"
	ReasonKilled              SessionStateReason = "killed"
	ReasonUserInterrupt       SessionStateReason = "user_interrupt"
	ReasonIdleReaped          SessionStateReason = "idle_reaped"
	ReasonPTYExited           SessionStateReason = "pty_exited"
	ReasonWatchdogNoProcess   SessionStateReason = "watchdog_no_process"
	ReasonTaskCompleted       SessionStateReason = "task_completed"
	ReasonTaskFailed          SessionStateReason = "task_failed"
	ReasonTaskCancelled       SessionStateReason = "task_cancelled"
	ReasonParentProcessExited SessionStateReason = "parent_process_exited"
	ReasonStartupReconcile    SessionStateReason = "startup_reconcile"
)

// SessionStateReasonDescription is one entry of the reason vocabulary as a
// client reads it.
type SessionStateReasonDescription struct {
	Reason  SessionStateReason `json:"reason"`
	Summary string             `json:"summary"`
}

// SessionStateReasons is the whole vocabulary, in the order above. It is the
// one list: llm-bridge-server serves it on GET /session-taxonomy, and
// ValidSessionStateReason reads it.
func SessionStateReasons() []SessionStateReasonDescription {
	return []SessionStateReasonDescription{
		{ReasonTurnStarted, "a message opened a turn"},
		{ReasonToolCalled, "the model called a tool"},
		{ReasonToolsDrained, "the last tool in flight answered, and the model carries on"},
		{ReasonCompactRequested, "the harness accepted a /compact"},
		{ReasonHarnessCompacting, "the harness began compacting on its own"},
		{ReasonCompactComplete, "a compaction finished"},
		{ReasonBackgroundTasksRunning, "the turn had ended, and the harness reported work still running in its process"},
		{ReasonBackgroundTasksFinished, "the last background task in the harness process finished"},
		{ReasonPermissionRequested, "a permission prompt opened and waits on a person"},
		{ReasonPermissionResolved, "every open permission prompt was answered"},
		{ReasonApprovalRequired, "a legacy approval prompt opened"},
		{ReasonApprovalResolved, "a legacy approval prompt was answered"},
		{ReasonTurnComplete, "the turn ended"},
		{ReasonTurnCompleteAwaitingUser, "the turn ended on what reads as a question to the user"},
		{ReasonTurnCompleteBackgroundTasksRunning, "the turn ended with work still running in the harness process"},
		{ReasonHarnessError, "the harness reported an error"},
		{ReasonAborted, "an aborted state passed through the event stream"},
		{ReasonLifecycle, "a lifecycle state the server sent through the event stream"},
		{ReasonTurnSettledReconcile, "the stored state was stale after the turn ended, and was corrected"},
		{ReasonTurnCompleteSignalQuestion, "the turn-end classifier found a question for the user"},
		{ReasonTurnCompleteSignalNotification, "the turn-end classifier found a notification, which asks nothing"},
		{ReasonTurnCompleteSignalNeither, "the turn-end classifier found nothing addressed to the user"},
		{ReasonSignalDismissed, "a person dismissed the question the session was waiting on"},
		{ReasonMarkDoneUser, "a person marked the session done, or not done"},
		{ReasonProcessSpawned, "the server started a harness process"},
		{ReasonSpawnFailed, "the server could not start a harness process; reason_detail says why"},
		{ReasonKilled, "someone stopped the session"},
		{ReasonUserInterrupt, "a person interrupted the turn"},
		{ReasonIdleReaped, "the idle reaper killed the quiet harness process to free its memory; a message starts it again"},
		{ReasonPTYExited, "the terminal-mode harness process exited"},
		{ReasonWatchdogNoProcess, "the session read as working but had no process, so the server reset it to resume it"},
		{ReasonTaskCompleted, "the parent reported this subagent's task completed"},
		{ReasonTaskFailed, "the parent reported this subagent's task failed"},
		{ReasonTaskCancelled, "the parent reported this subagent's task cancelled"},
		{ReasonParentProcessExited, "the process this subagent ran inside is gone, and its task never reported an end"},
		{ReasonStartupReconcile, "the server restarted while the session was working, and reset it"},
	}
}

// ValidSessionStateReason reports whether r is in the vocabulary.
func ValidSessionStateReason(r SessionStateReason) bool {
	for _, d := range SessionStateReasons() {
		if d.Reason == r {
			return true
		}
	}
	return false
}
