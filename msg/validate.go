package msg

import (
	"fmt"
	"strings"
)

// ValidationError contains one or more validation failures.
type ValidationError struct {
	Failures []ValidationFailure
}

func (e *ValidationError) Error() string {
	if len(e.Failures) == 1 {
		return e.Failures[0].String()
	}
	var parts []string
	for _, f := range e.Failures {
		parts = append(parts, f.String())
	}
	return fmt.Sprintf("%d validation failures: %s", len(e.Failures), strings.Join(parts, "; "))
}

// ValidationFailure describes a single validation issue.
type ValidationFailure struct {
	Path    string
	Code    string
	Message string
}

func (f ValidationFailure) String() string {
	if f.Path != "" {
		return fmt.Sprintf("%s: %s (%s)", f.Path, f.Message, f.Code)
	}
	return fmt.Sprintf("%s (%s)", f.Message, f.Code)
}

// Validation error codes — conversation/response.
const (
	ErrEmptyField           = "empty_field"
	ErrInvalidRole          = "invalid_role"
	ErrInvalidBlockType     = "invalid_block_type"
	ErrMultipleBlocksSet    = "multiple_blocks_set"
	ErrNoBlockSet           = "no_block_set"
	ErrMissingToolID        = "missing_tool_id"
	ErrMissingToolName      = "missing_tool_name"
	ErrOrphanedToolResult   = "orphaned_tool_result"
	ErrAlternationViolation = "alternation_violation"
	ErrMissingModel         = "missing_model"
	ErrMissingProvider      = "missing_provider"
	ErrInvalidStopReason    = "invalid_stop_reason"
	ErrThinkingNoSignature  = "thinking_no_signature"
	ErrOpaqueDataModified   = "opaque_data_modified"
)

// Validation error codes — event/session.
const (
	ErrMissingSessionID    = "missing_session_id"
	ErrMissingHarness      = "missing_harness"
	ErrInvalidEventType    = "invalid_event_type"
	ErrNoEventPayload      = "no_event_payload"
	ErrMultiplePayloads    = "multiple_payloads"
	ErrInvalidSessionState = "invalid_session_state"
	ErrMissingTimestamp    = "missing_timestamp"
)

// --- Conversation/Response validation ---

// ValidateConversation checks a Conversation for structural correctness.
func ValidateConversation(c *Conversation) *ValidationError {
	var failures []ValidationFailure

	if c.Model == "" {
		failures = append(failures, ValidationFailure{
			Path: "model", Code: ErrMissingModel, Message: "model is required",
		})
	}
	if c.Provider == "" {
		failures = append(failures, ValidationFailure{
			Path: "provider", Code: ErrMissingProvider, Message: "provider is required",
		})
	}

	for i, block := range c.System {
		if err := block.Validate(); err != nil {
			failures = append(failures, ValidationFailure{
				Path: fmt.Sprintf("system[%d]", i), Code: ErrInvalidBlockType, Message: err.Error(),
			})
		}
	}

	for i, msg := range c.Messages {
		msgFailures := validateMessage(&msg, fmt.Sprintf("messages[%d]", i))
		failures = append(failures, msgFailures...)
	}

	failures = append(failures, validateToolConsistency(c.Messages)...)

	if c.Provider == ProviderAnthropic {
		failures = append(failures, validateAlternation(c.Messages)...)
	}

	if len(failures) == 0 {
		return nil
	}
	return &ValidationError{Failures: failures}
}

// ValidateResponse checks a CompletionResponse for structural correctness.
func ValidateResponse(r *CompletionResponse) *ValidationError {
	var failures []ValidationFailure

	if r.ID == "" {
		failures = append(failures, ValidationFailure{
			Path: "id", Code: ErrEmptyField, Message: "response ID is required",
		})
	}
	if r.Model == "" {
		failures = append(failures, ValidationFailure{
			Path: "model", Code: ErrMissingModel, Message: "model is required",
		})
	}

	switch r.StopReason {
	case StopEnd, StopMaxTokens, StopToolUse, StopStopSequence, StopSafety, StopContentFilter, StopRecitation, StopError:
		// Valid.
	case "":
		// Can be empty during streaming.
	default:
		failures = append(failures, ValidationFailure{
			Path: "stop_reason", Code: ErrInvalidStopReason,
			Message: fmt.Sprintf("unknown stop reason %q", r.StopReason),
		})
	}

	msgFailures := validateMessage(&r.Message, "message")
	failures = append(failures, msgFailures...)

	if len(failures) == 0 {
		return nil
	}
	return &ValidationError{Failures: failures}
}

// --- Event/Session validation ---

// ValidateEvent checks an Event for structural correctness.
func ValidateEvent(e *Event) *ValidationError {
	var failures []ValidationFailure

	if e.Harness == "" {
		failures = append(failures, ValidationFailure{
			Path: "harness", Code: ErrMissingHarness, Message: "harness is required",
		})
	}
	if e.SessionID == "" {
		failures = append(failures, ValidationFailure{
			Path: "session_id", Code: ErrMissingSessionID, Message: "session_id is required",
		})
	}
	if e.Timestamp.IsZero() {
		failures = append(failures, ValidationFailure{
			Path: "timestamp", Code: ErrMissingTimestamp, Message: "timestamp is required",
		})
	}

	switch e.Type {
	case EventResult, EventStream, EventToolCall, EventToolResult,
		EventThinking, EventSystem, EventApproval, EventError, EventSessionState, EventPlan, EventHook:
		// Valid.
	case "":
		failures = append(failures, ValidationFailure{
			Path: "type", Code: ErrInvalidEventType, Message: "event type is required",
		})
	default:
		failures = append(failures, ValidationFailure{
			Path: "type", Code: ErrInvalidEventType,
			Message: fmt.Sprintf("unknown event type %q", e.Type),
		})
	}

	payloads := 0
	if e.Result != nil {
		payloads++
	}
	if e.Stream != nil {
		payloads++
	}
	if e.ToolCall != nil {
		payloads++
	}
	if e.ToolResult != nil {
		payloads++
	}
	if e.Thinking != nil {
		payloads++
	}
	if e.Plan != nil {
		payloads++
	}
	if e.System != nil {
		payloads++
	}
	if e.Approval != nil {
		payloads++
	}
	if e.Error != nil {
		payloads++
	}
	if e.State != nil {
		payloads++
	}
	if e.Hook != nil {
		payloads++
	}

	if payloads == 0 && e.Type != "" {
		failures = append(failures, ValidationFailure{
			Path: "", Code: ErrNoEventPayload,
			Message: fmt.Sprintf("event type %q has no payload", e.Type),
		})
	}
	if payloads > 1 {
		failures = append(failures, ValidationFailure{
			Path: "", Code: ErrMultiplePayloads,
			Message: fmt.Sprintf("event has %d payloads, expected 1", payloads),
		})
	}

	if len(failures) == 0 {
		return nil
	}
	return &ValidationError{Failures: failures}
}

// ValidateSession checks a Session for structural correctness.
func ValidateSession(s *Session) *ValidationError {
	var failures []ValidationFailure

	if s.ID == "" {
		failures = append(failures, ValidationFailure{
			Path: "id", Code: ErrMissingSessionID, Message: "session ID is required",
		})
	}
	if s.Harness == "" {
		failures = append(failures, ValidationFailure{
			Path: "harness", Code: ErrMissingHarness, Message: "harness is required",
		})
	}

	switch s.State {
	case SessionIdle, SessionRunning, SessionCompleted, SessionError, SessionAborted, SessionWaitingApproval:
		// Valid.
	case "":
		failures = append(failures, ValidationFailure{
			Path: "state", Code: ErrInvalidSessionState, Message: "session state is required",
		})
	default:
		failures = append(failures, ValidationFailure{
			Path: "state", Code: ErrInvalidSessionState,
			Message: fmt.Sprintf("unknown session state %q", s.State),
		})
	}

	if len(failures) == 0 {
		return nil
	}
	return &ValidationError{Failures: failures}
}

// --- Internal helpers ---

func validateMessage(m *Message, path string) []ValidationFailure {
	var failures []ValidationFailure

	switch m.Role {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		// Valid.
	case "":
		failures = append(failures, ValidationFailure{
			Path: path + ".role", Code: ErrEmptyField, Message: "role is required",
		})
	default:
		failures = append(failures, ValidationFailure{
			Path: path + ".role", Code: ErrInvalidRole,
			Message: fmt.Sprintf("unknown role %q", m.Role),
		})
	}

	for i, block := range m.Content {
		if err := block.Validate(); err != nil {
			failures = append(failures, ValidationFailure{
				Path: fmt.Sprintf("%s.content[%d]", path, i), Code: ErrInvalidBlockType,
				Message: err.Error(),
			})
		}

		if block.ToolUse != nil {
			if block.ToolUse.ID == "" && block.ToolUse.OriginalID == "" {
				failures = append(failures, ValidationFailure{
					Path: fmt.Sprintf("%s.content[%d].tool_use.id", path, i),
					Code: ErrMissingToolID, Message: "tool use block must have an ID",
				})
			}
			if block.ToolUse.Name == "" {
				failures = append(failures, ValidationFailure{
					Path: fmt.Sprintf("%s.content[%d].tool_use.name", path, i),
					Code: ErrMissingToolName, Message: "tool use block must have a name",
				})
			}
		}

		if block.ToolResult != nil {
			if block.ToolResult.ToolUseID == "" && block.ToolResult.OriginalToolUseID == "" {
				failures = append(failures, ValidationFailure{
					Path: fmt.Sprintf("%s.content[%d].tool_result.tool_use_id", path, i),
					Code: ErrMissingToolID, Message: "tool result must reference a tool use ID",
				})
			}
		}

		if block.Thinking != nil && block.Thinking.Signature == "" && block.Thinking.Text != "" {
			failures = append(failures, ValidationFailure{
				Path: fmt.Sprintf("%s.content[%d].thinking.signature", path, i),
				Code: ErrThinkingNoSignature,
				Message: "thinking block has text but no signature (may break Anthropic round-trip)",
			})
		}
	}

	return failures
}

func validateToolConsistency(messages []Message) []ValidationFailure {
	var failures []ValidationFailure
	toolUseIDs := map[string]bool{}

	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.ToolUse != nil {
				id := block.ToolUse.ID
				if id == "" {
					id = block.ToolUse.OriginalID
				}
				toolUseIDs[id] = true
			}
		}
	}

	for i, msg := range messages {
		for j, block := range msg.Content {
			if block.ToolResult != nil {
				id := block.ToolResult.ToolUseID
				if id == "" {
					id = block.ToolResult.OriginalToolUseID
				}
				if id != "" && !toolUseIDs[id] {
					failures = append(failures, ValidationFailure{
						Path: fmt.Sprintf("messages[%d].content[%d].tool_result.tool_use_id", i, j),
						Code: ErrOrphanedToolResult,
						Message: fmt.Sprintf("tool result references unknown tool use %q", id),
					})
				}
			}
		}
	}

	return failures
}

func validateAlternation(messages []Message) []ValidationFailure {
	var failures []ValidationFailure
	var prevRole Role
	for i, msg := range messages {
		if msg.Role == RoleSystem {
			continue
		}
		role := msg.Role
		if role == RoleTool {
			role = RoleUser
		}
		if prevRole != "" && role == prevRole {
			failures = append(failures, ValidationFailure{
				Path: fmt.Sprintf("messages[%d].role", i),
				Code: ErrAlternationViolation,
				Message: fmt.Sprintf("consecutive %s messages violate alternation (Anthropic requires user/assistant alternation)", role),
			})
		}
		prevRole = role
	}
	return failures
}
