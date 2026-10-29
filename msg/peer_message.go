package msg

import (
	"time"
)

// Messages between connected bridge servers: one server asks the other for
// work (a request), says a change is ready to take (a notice), or reports how
// either went (a status). A message goes into its sender's outbox and is sent
// over the connection until the other server acknowledges it with its own id;
// both servers keep a record. An incoming request or notice waits for an
// administrator of the receiving server, who approves or declines it, and the
// receiver answers with a status. Text in a message is untrusted: it comes
// from another machine.

// PeerMessageKind is what a message is.
type PeerMessageKind string

const (
	// PeerMessageKindRequest asks the other server to start a session; on
	// approval its prompt starts one there, framed as a request from another
	// machine.
	PeerMessageKindRequest PeerMessageKind = "request"
	// PeerMessageKindNotice says changes are ready to take; on approval the
	// receiver runs its own update.
	PeerMessageKindNotice PeerMessageKind = "notice"
	// PeerMessageKindStatus reports how a request or notice went. It needs no
	// approval.
	PeerMessageKindStatus PeerMessageKind = "status"
)

// PeerMessageKinds is every kind, for validation.
var PeerMessageKinds = []PeerMessageKind{PeerMessageKindRequest, PeerMessageKindNotice, PeerMessageKindStatus}

// PeerMessageDirection says which way a message went, from the point of view
// of the server holding the record.
type PeerMessageDirection string

const (
	PeerMessageDirectionIncoming PeerMessageDirection = "incoming"
	PeerMessageDirectionOutgoing PeerMessageDirection = "outgoing"
)

// PeerMessageState is where a message stands. An outgoing one is queued,
// delivered or failed; an incoming one is pending, approved, declined or done.
type PeerMessageState string

const (
	PeerMessageStateQueued    PeerMessageState = "queued"
	PeerMessageStateDelivered PeerMessageState = "delivered"
	PeerMessageStateFailed    PeerMessageState = "failed"
	PeerMessageStatePending   PeerMessageState = "pending"
	PeerMessageStateApproved  PeerMessageState = "approved"
	PeerMessageStateDeclined  PeerMessageState = "declined"
	PeerMessageStateDone      PeerMessageState = "done"
)

// PeerMessageStates is every state, for validation.
var PeerMessageStates = []PeerMessageState{
	PeerMessageStateQueued, PeerMessageStateDelivered, PeerMessageStateFailed,
	PeerMessageStatePending, PeerMessageStateApproved, PeerMessageStateDeclined, PeerMessageStateDone,
}

// PeerRepoCommit names one commit of one repo.
type PeerRepoCommit struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
}

// PeerMessageStatusResult is how a request or notice went.
type PeerMessageStatusResult string

const (
	PeerMessageStatusSucceeded PeerMessageStatusResult = "succeeded"
	PeerMessageStatusFailed    PeerMessageStatusResult = "failed"
	PeerMessageStatusDeclined  PeerMessageStatusResult = "declined"
)

// PeerMessageStatusResults is every result, for validation.
var PeerMessageStatusResults = []PeerMessageStatusResult{PeerMessageStatusSucceeded, PeerMessageStatusFailed, PeerMessageStatusDeclined}

// PeerMessageBody is what a message says. Which fields a kind carries:
//
//   - request: Title (the session it would start) and Prompt (its first
//     message);
//   - notice: Summary (what changed) and Commits (what is ready to take);
//   - status: Result, Detail (a decline's reason, the approved session's final
//     reply, or the update's own report) and, after an update, Versions (the
//     commits it reached).
//
// A field another kind carries is refused, not ignored.
type PeerMessageBody struct {
	Title    string                  `json:"title,omitempty"`
	Prompt   string                  `json:"prompt,omitempty"`
	Summary  string                  `json:"summary,omitempty"`
	Commits  []PeerRepoCommit        `json:"commits,omitempty"`
	Result   PeerMessageStatusResult `json:"result,omitempty"`
	Detail   string                  `json:"detail,omitempty"`
	Versions []PeerRepoCommit        `json:"versions,omitempty"`
}

// PeerMessage is one message as the server holding it records it.
//
// ID is this server's own id for it (peer_message_000001); the other server
// has its own, RemoteMessageID. InReplyTo is the id, on this server, of the
// message this one answers: a server sends the other's id for it on the wire,
// which is the receiver's own, so each side's InReplyTo is always one of its
// own ids and a conversation threads on both.
type PeerMessage struct {
	ID              string               `json:"id"`
	ConnectionID    string               `json:"connection_id"`
	Direction       PeerMessageDirection `json:"direction"`
	Kind            PeerMessageKind      `json:"kind"`
	Body            PeerMessageBody      `json:"body"`
	InReplyTo       string               `json:"in_reply_to,omitempty"`
	RemoteMessageID string               `json:"remote_message_id,omitempty"`
	State           PeerMessageState     `json:"state"`
	// CreatedBy is the administrator, or `session:<id>` for a session's agent,
	// who sent an outgoing message; empty for an incoming one.
	CreatedBy string `json:"created_by,omitempty"`
	// SenderServerName is the other server's name when an incoming message
	// arrived, for display; join on ConnectionID.
	SenderServerName string     `json:"sender_server_name,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeliveredAt      *time.Time `json:"delivered_at,omitempty"`
	// Attempts and LastError are an outgoing message's delivery so far;
	// NextAttemptAt is when it is tried again while queued.
	Attempts      int        `json:"attempts,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	// DecidedBy, DecidedAt and DecisionReason are an incoming message's
	// approval or decline.
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecisionReason string     `json:"decision_reason,omitempty"`
	// SessionID is the session an approved request started (or the
	// orchestrator's home session it was handed to).
	SessionID string `json:"session_id,omitempty"`
	// Operation is the update an approved notice ran.
	Operation *PeerMessageOperation `json:"operation,omitempty"`
	// StatusMessageID is the outgoing status this server sent in answer to
	// an incoming request or notice.
	StatusMessageID string `json:"status_message_id,omitempty"`
	// SignalID is the signal that put an incoming message in front of an
	// administrator.
	SignalID string `json:"signal_id,omitempty"`
}

// PeerMessageOperation is the update an approved notice ran: the command was
// started detached, writes its result to ResultPath and its output to
// LogPath, and Result is what it reported.
type PeerMessageOperation struct {
	ID         string           `json:"id"`
	StartedAt  time.Time        `json:"started_at"`
	ResultPath string           `json:"result_path"`
	LogPath    string           `json:"log_path"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Result     *PeerMessageBody `json:"result,omitempty"`
}

// PeerMessageCreateRequest is POST /peer-connections/{id}/messages. InReplyTo
// is the id, on this server, of the message this one answers.
type PeerMessageCreateRequest struct {
	Kind      PeerMessageKind `json:"kind"`
	Body      PeerMessageBody `json:"body"`
	InReplyTo string          `json:"in_reply_to,omitempty"`
}

// PeerMessageDelivery is POST /peer/messages, the connection call that hands
// a message to the other server. ID is the sender's id; InReplyTo is the
// receiver's own id of the message it answers.
type PeerMessageDelivery struct {
	ID        string          `json:"id"`
	Kind      PeerMessageKind `json:"kind"`
	Body      PeerMessageBody `json:"body"`
	InReplyTo string          `json:"in_reply_to,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// PeerMessageAcknowledgement is the receiver's answer to a delivery: its own
// id for the message.
type PeerMessageAcknowledgement struct {
	ID string `json:"id"`
}

// PeerMessageHandoffOrchestrator hands an approved request to the
// orchestrator instead of starting a session on an instance.
const PeerMessageHandoffOrchestrator = "orchestrator"

// PeerMessageApproveRequest is POST /peer-messages/{id}/approve. A request
// takes InstanceID or Handoff; a notice takes neither.
type PeerMessageApproveRequest struct {
	InstanceID string `json:"instance_id,omitempty"`
	Handoff    string `json:"handoff,omitempty"`
}

// PeerMessageDeclineRequest is POST /peer-messages/{id}/decline.
type PeerMessageDeclineRequest struct {
	Reason string `json:"reason"`
}

// SelfUpdateResultFileEnvironmentVariable and its two siblings are what the
// self_update.command is run with: where to write its result (a
// PeerMessageBody with Result, Detail and Versions, as JSON), the operation's id, and the notice's commits
// as JSON (a list of PeerRepoCommit).
const (
	SelfUpdateResultFileEnvironmentVariable  = "LLMBRIDGE_SELF_UPDATE_RESULT_FILE"
	SelfUpdateOperationIDEnvironmentVariable = "LLMBRIDGE_SELF_UPDATE_OPERATION_ID"
	SelfUpdateCommitsEnvironmentVariable     = "LLMBRIDGE_SELF_UPDATE_COMMITS"
)
