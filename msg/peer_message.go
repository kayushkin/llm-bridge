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
	// PeerMessageKindPatch offers a change to one repo as `git format-patch`
	// files, attached to the message. On approval a session lands it on the
	// receiving server.
	PeerMessageKindPatch PeerMessageKind = "patch"
)

// PeerMessageKinds is every kind, for validation.
var PeerMessageKinds = []PeerMessageKind{PeerMessageKindRequest, PeerMessageKindNotice, PeerMessageKindStatus, PeerMessageKindPatch}

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
//     commits it reached);
//   - patch: Repo (which repo it changes), BaseCommit (the commit it was made
//     against) and Summary; the patch files themselves are attachments.
//
// A field another kind carries is refused, not ignored. Any kind may carry
// attachments; a patch needs at least one.
type PeerMessageBody struct {
	Title      string                  `json:"title,omitempty"`
	Prompt     string                  `json:"prompt,omitempty"`
	Summary    string                  `json:"summary,omitempty"`
	Commits    []PeerRepoCommit        `json:"commits,omitempty"`
	Result     PeerMessageStatusResult `json:"result,omitempty"`
	Detail     string                  `json:"detail,omitempty"`
	Versions   []PeerRepoCommit        `json:"versions,omitempty"`
	Repo       string                  `json:"repo,omitempty"`
	BaseCommit string                  `json:"base_commit,omitempty"`
}

// PeerMessageAttachmentState is where an attachment's bytes stand on the
// server holding the record. An outgoing one is stored (its sender keeps the
// bytes for the receiver to fetch); an incoming one is pending until its bytes
// arrive and match their size and SHA-256, then fetched, or failed after the
// fetch gave up.
type PeerMessageAttachmentState string

const (
	PeerMessageAttachmentStored  PeerMessageAttachmentState = "stored"
	PeerMessageAttachmentPending PeerMessageAttachmentState = "pending"
	PeerMessageAttachmentFetched PeerMessageAttachmentState = "fetched"
	PeerMessageAttachmentFailed  PeerMessageAttachmentState = "failed"
)

// PeerMessageAttachmentDescription is an attachment as the sender describes
// it on the wire: ID is the sender's own id for it, which the receiver uses
// to fetch the bytes.
type PeerMessageAttachmentDescription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

// PeerMessageAttachment is one file a message carries, as the server holding
// it records it. ID is this server's own id (peer_attachment_000001);
// RemoteAttachmentID is the sender's, on an incoming one. The bytes never
// travel inside a message: the receiver fetches them over the connection
// after the message arrives, and keeps them only once their size and SHA-256
// match what the sender described. A server serves the bytes only as a
// download (application/octet-stream), never as a page.
type PeerMessageAttachment struct {
	ID                 string                     `json:"id"`
	MessageID          string                     `json:"message_id,omitempty"`
	Name               string                     `json:"name"`
	MediaType          string                     `json:"media_type"`
	Size               int64                      `json:"size"`
	SHA256             string                     `json:"sha256"`
	State              PeerMessageAttachmentState `json:"state"`
	RemoteAttachmentID string                     `json:"remote_attachment_id,omitempty"`
	Attempts           int                        `json:"attempts,omitempty"`
	LastError          string                     `json:"last_error,omitempty"`
	FetchedAt          *time.Time                 `json:"fetched_at,omitempty"`
	CreatedAt          time.Time                  `json:"created_at"`
}

// PeerPatchFileChange is one file a patch changes, as the receiving server
// read it from the verified patch files: never the sender's own account of it.
type PeerPatchFileChange struct {
	Path       string `json:"path"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// PeerPatchSummary is what the receiving server read from a patch's verified
// files: each commit's subject and the files they change.
type PeerPatchSummary struct {
	Subjects []string              `json:"subjects"`
	Files    []PeerPatchFileChange `json:"files"`
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
	// Attachments are the files the message carries.
	Attachments []PeerMessageAttachment `json:"attachments,omitempty"`
	// Patch is what an incoming patch's verified files change, read by this
	// server once every attachment is fetched.
	Patch *PeerPatchSummary `json:"patch,omitempty"`
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
//
// AttachmentIDs are files uploaded first with POST /peer-message-attachments
// by the same sender and not yet sent with another message.
type PeerMessageCreateRequest struct {
	Kind          PeerMessageKind `json:"kind"`
	Body          PeerMessageBody `json:"body"`
	InReplyTo     string          `json:"in_reply_to,omitempty"`
	AttachmentIDs []string        `json:"attachment_ids,omitempty"`
}

// PeerMessageDelivery is POST /peer/messages, the connection call that hands
// a message to the other server. ID is the sender's id; InReplyTo is the
// receiver's own id of the message it answers.
type PeerMessageDelivery struct {
	ID          string                             `json:"id"`
	Kind        PeerMessageKind                    `json:"kind"`
	Body        PeerMessageBody                    `json:"body"`
	InReplyTo   string                             `json:"in_reply_to,omitempty"`
	CreatedAt   time.Time                          `json:"created_at"`
	Attachments []PeerMessageAttachmentDescription `json:"attachments,omitempty"`
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
