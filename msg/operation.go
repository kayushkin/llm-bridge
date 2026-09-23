package msg

import (
	"encoding/json"
	"time"
)

// An operation is one piece of asynchronous work a caller asks the bridge to
// do: classify a batch of mail, apply an action to a ticket, run a query. The
// caller sends an OperationIntent, gets an OperationReceipt back at once, and
// follows the receipt to a terminal state by polling it or by streaming its
// OperationEvents. docs/OPERATIONS.md is the contract in prose: states,
// idempotency, correlation, redaction and retention.
//
// The common types here carry no domain fields — no ticket columns, no mail
// headers, no provider request bodies. What an operation of one type needs
// travels in Input, a payload whose shape the operation type defines, and in
// references to records other stores own. What it produced travels in Result,
// shaped the same way, and in ResultReferences.

// OperationType names what an operation does, as "<domain>.<verb>". The bridge
// runs only the types it has an executor for; GET /operation-types lists them.
type OperationType string

const (
	OperationTypeLLMCompletion     OperationType = "llm.completion"
	OperationTypeClassificationRun OperationType = "classification.run"
	OperationTypeTicketAction      OperationType = "ticket.action"
	OperationTypeDatabricksQuery   OperationType = "databricks.query"
	OperationTypeMailAction        OperationType = "mail.action"
)

// OperationState is where an operation is in its life. queued and running are
// the only states it can leave; the other five are terminal and never change.
type OperationState string

const (
	// OperationStateQueued is accepted and persisted, waiting for a worker.
	OperationStateQueued OperationState = "queued"
	// OperationStateRunning is held by a worker under a lease.
	OperationStateRunning OperationState = "running"
	// OperationStateSucceeded finished and did what it was asked.
	OperationStateSucceeded OperationState = "succeeded"
	// OperationStateFailed finished without doing what it was asked, and the
	// bridge knows no external effect happened, or knows which did.
	OperationStateFailed OperationState = "failed"
	// OperationStateConflicted was refused by the store that owns the record
	// it acted on, because the record had moved on (a stale row version, say).
	// Nothing was applied; the caller should refresh and decide again.
	OperationStateConflicted OperationState = "conflicted"
	// OperationStateUnknown may or may not have had its external effect: a
	// call timed out after the request left, and reconciliation could not
	// find out. Never retried automatically. Effects says which effect is in
	// doubt.
	OperationStateUnknown OperationState = "unknown"
	// OperationStateCancelled stopped at the caller's request before
	// finishing. Effects says whether anything was applied first.
	OperationStateCancelled OperationState = "cancelled"
)

// AllOperationStates lists every state, in lifecycle order.
var AllOperationStates = []OperationState{
	OperationStateQueued,
	OperationStateRunning,
	OperationStateSucceeded,
	OperationStateFailed,
	OperationStateConflicted,
	OperationStateUnknown,
	OperationStateCancelled,
}

// IsTerminal reports whether an operation in this state is finished for good.
func (state OperationState) IsTerminal() bool {
	switch state {
	case OperationStateSucceeded, OperationStateFailed, OperationStateConflicted,
		OperationStateUnknown, OperationStateCancelled:
		return true
	}
	return false
}

// IsKnown reports whether state is one of AllOperationStates.
func (state OperationState) IsKnown() bool {
	for _, known := range AllOperationStates {
		if state == known {
			return true
		}
	}
	return false
}

// OperationReference points at a record another store owns, by the id that
// store assigned. EntityType names the store's kind of record ("file",
// "card", "mail_message", "principal"); the pair is what a reader resolves.
// A reference never carries the record's content.
type OperationReference struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	// Version is the record's version when the reference was taken, for
	// stores that version their rows. Empty when the store does not.
	Version string `json:"version,omitempty"`
}

// OperationIntent is what a caller asks for. The bridge persists it before
// anything runs, so an accepted intent is never lost to a restart.
type OperationIntent struct {
	Type OperationType `json:"type"`
	// OrganizationID is the principal-store group the operation runs for
	// (principal_…). Required. A principal caller must be a member of it.
	OrganizationID string `json:"organization_id"`
	// PrincipalID is who asked. The bridge sets it from the caller's verified
	// identity and ignores what the body says; empty only when the internal
	// service calls on its own behalf.
	PrincipalID string `json:"principal_id,omitempty"`
	// IdempotencyKey is the caller's name for this request, unique per
	// principal, organization and type. Required. Sending the same key with
	// the same intent returns the first receipt; the same key with a
	// different intent is refused. See docs/OPERATIONS.md.
	IdempotencyKey string `json:"idempotency_key"`
	// CorrelationID ties the operation to the caller's own trace — a page
	// load, a batch, an upstream request. Passed unchanged to every store the
	// operation calls. The bridge sets it to the operation id when empty.
	CorrelationID string `json:"correlation_id,omitempty"`
	// ParentOperationID is set on a child operation, by the bridge. A caller
	// cannot set it.
	ParentOperationID string `json:"parent_operation_id,omitempty"`
	// Scope names the records the operation may read or change. An executor
	// refuses to touch a record outside it.
	Scope []OperationReference `json:"scope,omitempty"`
	// InputReferences names records the operation reads its input from, such
	// as a file in file-store, rather than carrying their content.
	InputReferences []OperationReference `json:"input_references,omitempty"`
	// Input is the payload the operation type defines. It must not carry
	// credentials; see docs/OPERATIONS.md.
	Input json.RawMessage `json:"input,omitempty"`
	// RequestedCapabilities names what the operation needs to be allowed to
	// do, beyond running its type ("mail.read", "ticket.write"). No
	// capability is defined yet, so any value is refused.
	RequestedCapabilities []string `json:"requested_capabilities,omitempty"`
	// MaximumCostUSD caps what this one operation may spend, children
	// included. Zero means no cap of its own; the organization's monthly
	// budget still applies.
	MaximumCostUSD float64 `json:"maximum_cost_usd,omitempty"`
	// PolicyRevision is the revision of the caller's policy the intent was
	// built under, recorded so a later reader can tell which rules applied.
	PolicyRevision string `json:"policy_revision,omitempty"`
}

// OperationProgress is how far a running operation has got, in units its type
// defines (items classified, pages read). Total is 0 while unknown.
type OperationProgress struct {
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Message   string `json:"message,omitempty"`
}

// OperationChildrenSummary counts an operation's children by state, so a
// batch can show aggregate progress without reading every child.
type OperationChildrenSummary struct {
	Total   int                    `json:"total"`
	ByState map[OperationState]int `json:"by_state"`
}

// OperationEvidence is one thing the result rests on: a matched phrase, a
// cited row, the model that answered. Detail is shaped by Kind.
type OperationEvidence struct {
	Kind      string              `json:"kind"`
	Summary   string              `json:"summary,omitempty"`
	Reference *OperationReference `json:"reference,omitempty"`
	Detail    json.RawMessage     `json:"detail,omitempty"`
}

// OperationEffectOutcome is what the bridge knows about one external effect.
type OperationEffectOutcome string

const (
	// OperationEffectOutcomeApplied: the owning store confirmed the change.
	OperationEffectOutcomeApplied OperationEffectOutcome = "applied"
	// OperationEffectOutcomeNotApplied: the owning store confirmed it did not
	// apply the change (refused it, or never received it).
	OperationEffectOutcomeNotApplied OperationEffectOutcome = "not_applied"
	// OperationEffectOutcomeUnknown: the request left and no answer came back
	// that says either way.
	OperationEffectOutcomeUnknown OperationEffectOutcome = "unknown"
)

// OperationEffect is one change the operation asked another system to make,
// written to the effect ledger before the request leaves and updated when the
// answer comes back. It is how a receipt says "this may have happened".
type OperationEffect struct {
	// Target is the record the effect changes.
	Target OperationReference `json:"target"`
	// Action names the change in the target store's own words.
	Action string `json:"action"`
	// ExternalOperationID is the id the target system gave the request (a
	// statement id, a provider message id), so reconciliation can ask about
	// it. Empty until the system answers with one.
	ExternalOperationID string                 `json:"external_operation_id,omitempty"`
	Outcome             OperationEffectOutcome `json:"outcome"`
	RecordedAt          time.Time              `json:"recorded_at"`
}

// OperationUsage is what the operation cost, summed over its model calls.
type OperationUsage struct {
	Tokens TokenUsage `json:"tokens"`
	Cost   Cost       `json:"cost"`
	// Calls counts the model calls summed here.
	Calls int `json:"calls"`
	// CostBasis says how Cost was worked out. Always
	// OperationCostBasisListPrice today.
	CostBasis OperationCostBasis `json:"cost_basis,omitempty"`
}

// OperationCostBasis says where an operation's dollar figure comes from.
type OperationCostBasis string

// OperationCostBasisListPrice: tokens multiplied by model-store's per-model
// input and output price. It is what the call would cost on a metered API
// key. A call on a subscription login costs nothing extra, so for those it is
// an upper bound, and budgets are enforced against it all the same.
const OperationCostBasisListPrice OperationCostBasis = "list_price"

// OrganizationBudget is one organization's monthly spending limit and what
// its operations have spent this month, as GET /operation-budgets serves it.
// Months are calendar months in UTC.
type OrganizationBudget struct {
	OrganizationID  string    `json:"organization_id"`
	MonthlyLimitUSD float64   `json:"monthly_limit_usd"`
	SpentUSD        float64   `json:"spent_usd"`
	MonthStartsAt   time.Time `json:"month_starts_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// UpdatedBy is the principal who set the limit, empty for the internal
	// service.
	UpdatedBy string `json:"updated_by,omitempty"`
}

// OperationError explains a failed, conflicted, unknown or cancelled
// operation. Code is stable and machine-readable; Message is for a person.
type OperationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Retryable says whether sending a new intent (with a new idempotency
	// key) could succeed. The bridge has already made every retry it will.
	Retryable bool `json:"retryable"`
}

// OperationReceipt is the bridge's authoritative account of one operation.
// Every read and every event carries the whole receipt, so a reader never has
// to assemble one from deltas.
type OperationReceipt struct {
	// ID is operation_<ULID>, assigned by the bridge.
	ID                string        `json:"id"`
	Type              OperationType `json:"type"`
	OrganizationID    string        `json:"organization_id"`
	PrincipalID       string        `json:"principal_id,omitempty"`
	IdempotencyKey    string        `json:"idempotency_key"`
	CorrelationID     string        `json:"correlation_id"`
	ParentOperationID string        `json:"parent_operation_id,omitempty"`

	State OperationState `json:"state"`
	// Revision rises by one with every change to the receipt. A reader keeps
	// the receipt with the highest revision it has seen.
	Revision int64 `json:"revision"`
	// Attempt counts how many times a worker has started the operation.
	Attempt  int                `json:"attempt"`
	Progress *OperationProgress `json:"progress,omitempty"`

	// Result is the payload the operation type defines, present once the
	// operation has succeeded.
	Result           json.RawMessage      `json:"result,omitempty"`
	ResultReferences []OperationReference `json:"result_references,omitempty"`
	Evidence         []OperationEvidence  `json:"evidence,omitempty"`
	Effects          []OperationEffect    `json:"effects,omitempty"`
	Usage            *OperationUsage      `json:"usage,omitempty"`
	Error            *OperationError      `json:"error,omitempty"`

	// Executor names what ran the operation, for diagnostics only. A product
	// surface must not branch on it.
	Executor string `json:"executor,omitempty"`

	ChildOperationIDs []string                  `json:"child_operation_ids,omitempty"`
	Children          *OperationChildrenSummary `json:"children,omitempty"`

	CancelRequestedAt *time.Time `json:"cancel_requested_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// OperationEventKind names what happened to an operation.
type OperationEventKind string

const (
	OperationEventAccepted     OperationEventKind = "accepted"
	OperationEventQueued       OperationEventKind = "queued"
	OperationEventStarted      OperationEventKind = "started"
	OperationEventProgress     OperationEventKind = "progress"
	OperationEventChildUpdated OperationEventKind = "child_updated"
	// OperationEventCancelRequested: the caller asked a running operation to
	// stop. It is still running until its executor returns.
	OperationEventCancelRequested OperationEventKind = "cancel_requested"
	OperationEventSucceeded    OperationEventKind = "succeeded"
	OperationEventFailed       OperationEventKind = "failed"
	OperationEventConflicted   OperationEventKind = "conflicted"
	OperationEventUnknown      OperationEventKind = "unknown"
	OperationEventCancelled    OperationEventKind = "cancelled"
)

// OperationEventKindForTerminalState is the event that announces an operation
// reaching a terminal state.
var OperationEventKindForTerminalState = map[OperationState]OperationEventKind{
	OperationStateSucceeded:  OperationEventSucceeded,
	OperationStateFailed:     OperationEventFailed,
	OperationStateConflicted: OperationEventConflicted,
	OperationStateUnknown:    OperationEventUnknown,
	OperationStateCancelled:  OperationEventCancelled,
}

// OperationEvent is one change to an operation, persisted and streamed in
// Sequence order. Every change to a receipt writes exactly one event, so
// Sequence equals Receipt.Revision. Receipt is the whole receipt after the
// change.
type OperationEvent struct {
	OperationID string             `json:"operation_id"`
	Sequence    int64              `json:"sequence"`
	Kind        OperationEventKind `json:"kind"`
	// ChildOperationID names the child that changed, on child_updated.
	ChildOperationID string           `json:"child_operation_id,omitempty"`
	Receipt          OperationReceipt `json:"receipt"`
	At               time.Time        `json:"at"`
}

// OperationTypeDescription is one operation type the bridge can run, as
// GET /operation-types serves it.
type OperationTypeDescription struct {
	Type        OperationType `json:"type"`
	Executor    string        `json:"executor"`
	Description string        `json:"description"`
	// MaximumAttempts is how many times a worker will start an operation of
	// this type before failing it.
	MaximumAttempts int `json:"maximum_attempts"`
	// TimeoutSeconds bounds one attempt.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// ClassificationRunInput is the Input of a classification.run operation.
type ClassificationRunInput struct {
	// Labels are the labels an item may be given. At least one.
	Labels []ClassificationLabel `json:"labels"`
	// Items are the texts to classify. At least one.
	Items []ClassificationItem `json:"items"`
}

// ClassificationLabel is one label a classifier may assign.
type ClassificationLabel struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Keywords are phrases that suggest the label. A model-backed classifier
	// may use them as hints; the keyword classifier uses nothing else.
	Keywords []string `json:"keywords,omitempty"`
}

// ClassificationItem is one text to classify. ID is the caller's id for it,
// ideally the owning store's (a mail message id), and comes back unchanged.
type ClassificationItem struct {
	ID        string              `json:"id"`
	Text      string              `json:"text"`
	Reference *OperationReference `json:"reference,omitempty"`
}

// ClassificationRunResult is the Result of a succeeded classification.run.
type ClassificationRunResult struct {
	Items []ClassificationItemResult `json:"items"`
}

// ClassificationItemResult is the classifier's answer for one item. Labels is
// empty when nothing matched; that is an answer, not a failure.
type ClassificationItemResult struct {
	ID     string   `json:"id"`
	Labels []string `json:"labels"`
	// Confidence is between 0 and 1.
	Confidence float64 `json:"confidence"`
	// Evidence is what the labels rest on, for this item.
	Evidence []OperationEvidence `json:"evidence,omitempty"`
}

// LLMCompletionInput is the Input of an llm.completion operation: one
// stateless model call through a bridge harness instance.
type LLMCompletionInput struct {
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	// Model asks for a model by model-store id. Empty takes the bridge's
	// configured completion model.
	Model string `json:"model,omitempty"`
	// Schema, when set, is a JSON Schema the answer must match; the answer
	// then arrives in Parsed.
	Schema    json.RawMessage `json:"schema,omitempty"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

// LLMCompletionResult is the Result of a succeeded llm.completion.
type LLMCompletionResult struct {
	Text       string          `json:"text,omitempty"`
	Parsed     json.RawMessage `json:"parsed,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
}
