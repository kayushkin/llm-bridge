package msg

import "time"

// Signal is the one canonical record for anything a session surfaces to a
// human: a question that needs an answer, or a notification that needs at
// most an acknowledgement. See SESSION-SIGNALS.md in llm-bridge-server for
// the full design.
//
// The record is unified across session type. What varies is Surface, which
// says which consumer renders the signal, and Kind, which says whether an
// answer is expected.
type Signal struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`

	// SessionType is metadata, not a discriminator. It records what kind of
	// session raised the signal; Surface (below) is what decides where the
	// signal renders.
	SessionType SessionType `json:"session_type,omitempty"`

	Kind   SignalKind   `json:"kind"`
	Source SignalSource `json:"source"`

	// RequestID pairs a tool-sourced signal with the parked hook request it
	// was minted from, so resolving the hook resolves the signal. Empty for
	// derived signals, which have no parked request behind them.
	RequestID string `json:"request_id,omitempty"`

	Surface SignalSurface `json:"surface"`

	// Title is the question text, or the notification headline.
	Title string `json:"title"`
	// Body is optional detail. Questions carry their detail in Options;
	// notifications use this for the paragraph under the headline.
	Body string `json:"body,omitempty"`

	// Options are the pre-baked answers offered for a question. Empty for
	// notifications.
	Options []SignalOption `json:"options,omitempty"`
	// AllowFreeform reports whether the resolve verb accepts a typed answer
	// instead of (or alongside) one of Options. Questions only.
	AllowFreeform bool `json:"allow_freeform,omitempty"`
	// AllowMultipleOptions reports whether more than one of Options may be
	// picked at once. Questions only.
	//
	// It is a property of the QUESTION, not of the producer that raised it:
	// AskUserQuestion sets it from its own multiSelect flag, and the derived
	// classifier leaves it false because it mints one-of-these questions. A
	// renderer needs it because the record is now what the answer form is
	// drawn from — without it a multi-select question renders as radio
	// buttons and the human can only send back one of the answers the model
	// asked for.
	AllowMultipleOptions bool `json:"allow_multiple_options,omitempty"`

	// Answer is filled in when a question is answered. Nil while open.
	Answer *SignalAnswer `json:"answer,omitempty"`

	// Severity is carried by notifications only.
	Severity SignalSeverity `json:"severity,omitempty"`

	State SignalState `json:"state"`

	// LinkedTodoID is the noteboard item this signal propagates to, when the
	// raising session is linked to one.
	LinkedTodoID string `json:"linked_todo_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	// ResolvedAt is stamped when State leaves open. Nil while open.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// SignalKind is the answer-expected axis. It is orthogonal to session type:
// an interactive session, a herald relay and an autonomous worker can each
// raise either kind.
type SignalKind string

const (
	// SignalKindQuestion needs an answer. It blocks: the raising session
	// sits at awaiting_user until the answer arrives.
	SignalKindQuestion SignalKind = "question"
	// SignalKindNotification needs at most an acknowledgement. It does not
	// block; the raising session keeps going.
	SignalKindNotification SignalKind = "notification"
)

// SignalSource records which producer minted the signal.
type SignalSource string

const (
	// SignalSourceTool means a structured tool call produced the signal —
	// AskUserQuestion for a question, a notify-style tool for a notification.
	SignalSourceTool SignalSource = "tool"
	// SignalSourceDerived means a classifier pass over a turn-end produced
	// the signal from free text.
	SignalSourceDerived SignalSource = "derived"
)

// SignalSurface is where a signal renders. It is a projection of
// attended-ness — whether a human is watching the raising session's chat —
// not of kind.
type SignalSurface string

const (
	// SignalSurfaceChat routes to the "Needs you" inbox, the raising
	// session's own chat, and reference chips. Attended sessions.
	SignalSurfaceChat SignalSurface = "chat"
	// SignalSurfaceKanban routes to the raising worker's kanban card, for
	// async review by the orchestrator, the user, or another agent.
	// Unattended sessions, whose chat nobody is reading.
	SignalSurfaceKanban SignalSurface = "kanban"
)

// SignalState is the lifecycle of one signal.
type SignalState string

const (
	SignalStateOpen         SignalState = "open"
	SignalStateAnswered     SignalState = "answered"     // question resolved with an answer
	SignalStateAcknowledged SignalState = "acknowledged" // notification seen
	SignalStateDismissed    SignalState = "dismissed"    // closed without an answer
)

// SignalSeverity grades a notification. Questions do not carry one.
type SignalSeverity string

const (
	SignalSeverityInfo SignalSeverity = "info"
	SignalSeverityWarn SignalSeverity = "warn"
)

// SignalOption is one pre-baked answer offered for a question.
type SignalOption struct {
	Label string `json:"label"`
	// Value is what the resolve verb sends back when this option is picked.
	// Producers that have no separate machine value set it to Label.
	Value string `json:"value"`
	// Description is the producer's longer gloss on the option, passed
	// through unchanged for the renderer to show.
	Description string `json:"description,omitempty"`
}

// SignalAnswer is a resolved question's answer. A picked option and a typed
// edit are not exclusive — the human may pick an option and amend it.
type SignalAnswer struct {
	Option string `json:"option,omitempty"`
	Text   string `json:"text,omitempty"`
}
