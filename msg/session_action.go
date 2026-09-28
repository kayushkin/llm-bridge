package msg

import "time"

// A session action is a button a session's agent puts in the chat. The person
// reading the chat confirms it, and llm-bridge-server does the work itself —
// no agent turn runs between the click and the effect. The agent offers one of
// a fixed set of action types; it never supplies a command to run.

// SessionActionType is what a session action does when a person confirms it.
type SessionActionType string

const (
	// SessionActionDeploy runs deploy.sh in a repo that repo-store knows.
	SessionActionDeploy SessionActionType = "deploy"
	// SessionActionRunSchedulerJob runs one scheduler job now.
	SessionActionRunSchedulerJob SessionActionType = "run_scheduler_job"
	// SessionActionSendMessage sent a message to the session the action is in,
	// as if the person had typed it. No longer offered since 2026-09-28:
	// agents used it for answer choices, which is the question feature's job
	// (AskUserQuestion draws each option as a one-click answer), so it only
	// repeated the question. Kept so the records made before then still read.
	SessionActionSendMessage SessionActionType = "send_message"
	// SessionActionForkAndSend forks the session and sends a message to the
	// fork, leaving the original session alone.
	SessionActionForkAndSend SessionActionType = "fork_and_send"
	// SessionActionNewSessionAndSend starts a new session with the original's
	// harness, instance, agent, principal and working directory, but none of its
	// history, and sends it a message.
	SessionActionNewSessionAndSend SessionActionType = "new_session_and_send"
)

// OfferableSessionActionTypes is every action type an agent may offer now,
// in the order a client lists them. Each one makes the server do something;
// none answers a question.
var OfferableSessionActionTypes = []SessionActionType{
	SessionActionDeploy,
	SessionActionRunSchedulerJob,
	SessionActionForkAndSend,
	SessionActionNewSessionAndSend,
}

// SessionActionState is where a session action is in its one run.
type SessionActionState string

const (
	// SessionActionOffered is waiting for a person to confirm it.
	SessionActionOffered SessionActionState = "offered"
	// SessionActionRunning has been confirmed and is running.
	SessionActionRunning SessionActionState = "running"
	// SessionActionSucceeded ran and did what it says.
	SessionActionSucceeded SessionActionState = "succeeded"
	// SessionActionFailed ran and did not; Error says why.
	SessionActionFailed SessionActionState = "failed"
	// SessionActionOutcomeUnknown was running when llm-bridge-server stopped,
	// so nobody saw how it ended.
	SessionActionOutcomeUnknown SessionActionState = "outcome_unknown"
)

// SessionActionOffer is what an agent sends to put a button in its session:
// POST /sessions/{id}/actions. Type decides which one of RepoID,
// SchedulerJobID and Message is required; the others must be empty. The
// agent then writes the action's id in its reply, and the chat draws the
// button there.
type SessionActionOffer struct {
	// Label is the button's text: what the action does, in the agent's words.
	Label string            `json:"label"`
	Type  SessionActionType `json:"type"`
	// RepoID is repo-store's id of the repo to deploy.
	RepoID int64 `json:"repo_id,omitempty"`
	// SchedulerJobID is the scheduler's id of the job to run.
	SchedulerJobID int64 `json:"scheduler_job_id,omitempty"`
	// Message is the text to send, for fork_and_send and new_session_and_send.
	Message string `json:"message,omitempty"`
}

// SessionAction is one button in a session and the record of its run. Every
// change to it is written to the session as an EventSessionAction carrying the
// whole record, so the chat draws the button where it was offered and each
// later event shows who ran it and how it ended.
type SessionAction struct {
	// ActionID is llm-bridge-server's id for the action (session_action_…).
	ActionID  string `json:"action_id"`
	SessionID string `json:"session_id"`
	// Offer is what the agent asked for, as it asked.
	Offer SessionActionOffer `json:"offer"`
	// Command says exactly what confirming the action runs, written by
	// llm-bridge-server from the offer when it was made: the directory and
	// script of a deploy, the URL of a scheduler job, the text of a message.
	Command   string             `json:"command"`
	State     SessionActionState `json:"state"`
	OfferedAt time.Time          `json:"offered_at"`
	// RunByPrincipalID is principal-store's id of the person who confirmed
	// it. Empty when the internal service confirmed it on no one's behalf.
	RunByPrincipalID string     `json:"run_by_principal_id,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	// Output is everything the run printed or answered: a deploy's combined
	// output, the scheduler's reply.
	Output string `json:"output,omitempty"`
	// Error says why a failed run failed.
	Error string `json:"error,omitempty"`
	// ResultSessionID is the session a fork_and_send or new_session_and_send
	// run created.
	ResultSessionID string `json:"result_session_id,omitempty"`
}
