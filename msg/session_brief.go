package msg

import "time"

// SessionBrief is what one chat session is for and where it stands, kept up to
// date by a cheap-model pass at the end of each of its turns.
//
// It answers the questions a person returning to a chat asks first: what did
// I ask for, what is it doing now, what happened along the way, and what did
// it say that I never answered. The first two live here; the history is the
// session's SessionBriefRevision rows, and the unanswered remarks are its
// LooseEnd rows.
//
// Only chats (interactive sessions) carry a brief. An autonomous worker has a
// kanban card for the same purpose.
type SessionBrief struct {
	SessionID string `json:"session_id"`
	// OverallGoal is what the person asked the session to do, in a sentence.
	// It changes only when the person changes what they want.
	OverallGoal string `json:"overall_goal"`
	// CurrentStep is what the session is doing now, or what it is waiting on.
	CurrentStep string `json:"current_step"`
	// RevisionCount is how many turns have updated the brief.
	RevisionCount int       `json:"revision_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SessionBriefRevision is the brief as one turn left it, kept so the brief
// has a history: a later turn never overwrites what an earlier one said.
type SessionBriefRevision struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	OverallGoal string `json:"overall_goal"`
	CurrentStep string `json:"current_step"`
	// GoalChanged is true when this turn changed the overall goal: the person
	// redirected the session.
	GoalChanged bool `json:"goal_changed"`
	// TurnChange is one line saying what the turn changed in the world —
	// a commit, a deploy, a file written, a setting changed — or empty when it
	// only talked.
	TurnChange string    `json:"turn_change,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// LooseEnd is something a chat session said that wanted the person's reply
// and never got one: a question asked mid-turn, an offer nobody took up, a
// problem it mentioned and then dropped.
//
// It is deliberately not a Signal. A session holds at most one open question
// signal, and any message the person sends closes it, because an answer lands
// at the end of the thread. A loose end is the opposite case: the thread moved
// on and the remark was left behind, so it stays open across messages until a
// later turn shows it was dealt with, or the person answers or dismisses it.
type LooseEnd struct {
	ID        string       `json:"id"`
	SessionID string       `json:"session_id"`
	Kind      LooseEndKind `json:"kind"`
	// Text is the remark, in the session's own words where possible.
	Text string `json:"text"`
	// Context is any short detail a person needs to act on it.
	Context string        `json:"context,omitempty"`
	State   LooseEndState `json:"state"`
	// Resolution says how it closed: the person's answer, or what the later
	// turn did that settled it.
	Resolution string     `json:"resolution,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// LooseEndKind says what sort of remark was left behind.
type LooseEndKind string

const (
	// LooseEndQuestion is a question to the person that was never answered.
	LooseEndQuestion LooseEndKind = "question"
	// LooseEndOffer is something the session offered to do that the person
	// neither took up nor turned down.
	LooseEndOffer LooseEndKind = "offer"
	// LooseEndDroppedProblem is a failure, risk or oddity the session
	// mentioned and did not follow up.
	LooseEndDroppedProblem LooseEndKind = "dropped_problem"
)

// LooseEndKinds lists every kind, for validation.
var LooseEndKinds = []LooseEndKind{LooseEndQuestion, LooseEndOffer, LooseEndDroppedProblem}

// LooseEndState is where a loose end stands.
type LooseEndState string

const (
	LooseEndOpen LooseEndState = "open"
	// LooseEndAnswered: the person answered it from the loose end itself.
	LooseEndAnswered LooseEndState = "answered"
	// LooseEndSettled: a later turn of the session dealt with it.
	LooseEndSettled LooseEndState = "settled"
	// LooseEndDismissed: the person closed it without an answer.
	LooseEndDismissed LooseEndState = "dismissed"
)

// LooseEndStates lists every state, for validation.
var LooseEndStates = []LooseEndState{LooseEndOpen, LooseEndAnswered, LooseEndSettled, LooseEndDismissed}

// SessionBriefView is one session's brief with its history and its loose
// ends, as GET /sessions/{id}/brief and GET /session-briefs answer.
type SessionBriefView struct {
	// Brief is nil for a session no turn has briefed yet.
	Brief *SessionBrief `json:"brief"`
	// Revisions are oldest first.
	Revisions []SessionBriefRevision `json:"revisions"`
	// LooseEnds are oldest first, open and closed alike unless the request
	// narrowed them.
	LooseEnds []LooseEnd `json:"loose_ends"`
}
