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
	// SessionActionRunCommand runs a shell command the agent wrote, in a
	// directory, and shows its output under the button. A reviewer model reads
	// the command when it is offered; a command it rejects cannot be run.
	SessionActionRunCommand SessionActionType = "run_command"
	// SessionActionModelCall asks a model one question, with no tools, and
	// shows its answer under the button.
	SessionActionModelCall SessionActionType = "model_call"
	// SessionActionBackgroundAgent starts a session set up like this one, with
	// a spend ceiling, sends it a task, and shows its final reply under the
	// button while the session itself stays open to look at.
	SessionActionBackgroundAgent SessionActionType = "background_agent"
	// SessionActionSendToSession sends a message to another session, named by
	// TargetSessionID, as if the person had typed it there. It lets an agent
	// that manages other sessions offer the next step in one of them.
	SessionActionSendToSession SessionActionType = "send_to_session"
	// SessionActionStopSession stops another session, named by
	// TargetSessionID: kills its harness process and marks it aborted.
	SessionActionStopSession SessionActionType = "stop_session"
)

// OfferableSessionActionTypes is every action type an agent may offer now,
// in the order a client lists them. Each one makes the server do something;
// none answers a question.
var OfferableSessionActionTypes = []SessionActionType{
	SessionActionDeploy,
	SessionActionRunSchedulerJob,
	SessionActionRunCommand,
	SessionActionModelCall,
	SessionActionBackgroundAgent,
	SessionActionForkAndSend,
	SessionActionNewSessionAndSend,
	SessionActionSendToSession,
	SessionActionStopSession,
}

// SessionActionResultFormat says how the chat draws an action's output.
type SessionActionResultFormat string

const (
	// SessionActionResultText is drawn as it came, in a fixed-width font.
	SessionActionResultText SessionActionResultFormat = "text"
	// SessionActionResultMarkdown is drawn as markdown: lists, tables, links,
	// and record ids as chips.
	SessionActionResultMarkdown SessionActionResultFormat = "markdown"
)

// SessionActionReviewVerdict is what the reviewer model made of a command.
type SessionActionReviewVerdict string

const (
	// SessionActionReviewApprove: the command does what its label says and
	// nothing it does is hard to undo.
	SessionActionReviewApprove SessionActionReviewVerdict = "approve"
	// SessionActionReviewCaution: it does what its label says, but it changes
	// something that matters — pushes, deploys, deletes, sends, spends. The
	// person should read it before confirming.
	SessionActionReviewCaution SessionActionReviewVerdict = "caution"
	// SessionActionReviewReject: it does not do what its label says, or it
	// could do harm the label does not admit. It cannot be run.
	SessionActionReviewReject SessionActionReviewVerdict = "reject"
)

// SessionActionReview is the reviewer model's reading of a command, made when
// the action was offered.
type SessionActionReview struct {
	Verdict SessionActionReviewVerdict `json:"verdict"`
	// Reasons is the reviewer's explanation, in a few sentences.
	Reasons string `json:"reasons"`
	// Model is the model that reviewed it, as model-store resolved it.
	Model      string    `json:"model"`
	ReviewedAt time.Time `json:"reviewed_at"`
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
// POST /sessions/{id}/actions. Type decides which fields are required and
// which are allowed; a field the type does not use must be empty. The
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
	// Message is the text to send, for fork_and_send, new_session_and_send,
	// send_to_session and background_agent, and the question for model_call.
	Message string `json:"message,omitempty"`
	// TargetSessionID is llm-bridge-server's id of the session a
	// send_to_session or stop_session acts on. It cannot be the session the
	// button is offered in.
	TargetSessionID string `json:"target_session_id,omitempty"`
	// ShellCommand is the command run_command runs with bash -l -c.
	ShellCommand string `json:"shell_command,omitempty"`
	// WorkingDirectory is where run_command runs; empty means the session's
	// own working directory.
	WorkingDirectory string `json:"working_directory,omitempty"`
	// ResultFormat is how the chat draws run_command's output; empty is text.
	// model_call and background_agent answers are always markdown.
	ResultFormat SessionActionResultFormat `json:"result_format,omitempty"`
	// Model names the model for model_call (required) or background_agent
	// (empty keeps this session's): a model-store id, alias or role.
	Model string `json:"model,omitempty"`
	// MaximumCostUSD is the most a model_call or background_agent may spend,
	// at list price. Required for both; the agent that offers the button sets
	// it, and the button shows it.
	MaximumCostUSD float64 `json:"maximum_cost_usd,omitempty"`
	// Inputs are text boxes a run_command or send_to_session button shows.
	// What the person types reaches a command only as environment variables,
	// never in its text; a send_to_session message names each input as
	// $BRIDGE_INPUT_<Name>, and the typed value takes its place in the message
	// sent.
	Inputs []SessionActionInput `json:"inputs,omitempty"`
	// ReportSuccess says whether the agent is told when a person's press of
	// the button succeeds. A failure, or an outcome nobody saw, is always
	// reported. Left empty, it takes SessionActionReportsSuccessByDefault for
	// the type, and the server stores the value it settled on.
	ReportSuccess *bool `json:"report_success,omitempty"`
}

// SessionActionReportsSuccessByDefault says whether a button of this type
// tells its agent about a successful press when the offer leaves
// ReportSuccess empty. A run_command or run_scheduler_job press is almost
// always a step the agent waits on (12 of the 13 successful run_command
// presses up to 2026-10-21 were), so it reports. Every other type shows its
// result under the button or works in another session, and none of the 12
// successful presses of those types gave the agent anything to act on. A
// deploy never reports here: deploy-gate tells the session that started it.
func SessionActionReportsSuccessByDefault(actionType SessionActionType) bool {
	return actionType == SessionActionRunCommand || actionType == SessionActionRunSchedulerJob
}

// A bridge report is a message the server, not the person, sends into a
// session: a button's outcome, how a deploy the session started ended, or that
// the harness was restarted under the session.
// It travels as a user message, because that is the only way to start the
// agent's turn, so the message carries its own mark: its whole text is one
// <bridge-report kind="…">…</bridge-report> block, the same way Claude Code
// marks its own <task-notification> messages. Chat clients draw it as a
// folded notice rather than as something the person wrote.
const (
	BridgeReportKindSessionAction = "session-action"
	BridgeReportKindDeploy        = "deploy"
	BridgeReportKindRestart       = "restart"
)

// WrapBridgeReport marks text as a bridge report of the given kind.
func WrapBridgeReport(kind, text string) string {
	return `<bridge-report kind="` + kind + `">` + "\n" + text + "\n</bridge-report>"
}

// SessionActionInputEnvironmentPrefix starts the environment variable an
// input's value reaches the command in: input PASSWORD is $BRIDGE_INPUT_PASSWORD.
const SessionActionInputEnvironmentPrefix = "BRIDGE_INPUT_"

// SessionActionInput is one text box on a button. The value typed into it is
// sent with the confirm and handed to the command as an environment variable;
// llm-bridge-server stores it nowhere and writes it into no log or event, so
// a password typed here goes from the person to the command and no further.
type SessionActionInput struct {
	// Name is upper case, digits and underscores, starting with a letter, at
	// most 32 characters; the value arrives as $BRIDGE_INPUT_<Name>.
	Name string `json:"name"`
	// Label is the text beside the box.
	Label string `json:"label"`
	// Secret draws the box as a password field, and the server blanks the
	// value wherever it appears in the command's output. Only run_command
	// takes one: a send_to_session value is written into the target's
	// conversation, so it can be no secret.
	Secret bool `json:"secret,omitempty"`
}

// SessionActionRunRequest is the body of a confirm,
// POST /sessions/{id}/actions/{action_id}/run: a value for each of the
// offer's inputs, by name. A button with no inputs needs no body.
type SessionActionRunRequest struct {
	Inputs map[string]string `json:"inputs,omitempty"`
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
	// ResultSessionID is the session a fork_and_send, new_session_and_send or
	// background_agent run created, set as soon as it exists.
	ResultSessionID string `json:"result_session_id,omitempty"`
	// Review is the reviewer model's reading of a run_command, made when it
	// was offered.
	Review *SessionActionReview `json:"review,omitempty"`
	// CostUSD is what a model_call spent, at list price.
	CostUSD float64 `json:"cost_usd,omitempty"`
}
