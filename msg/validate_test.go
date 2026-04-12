package msg

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- ValidateConversation ---

func TestValidateConversation_Valid(t *testing.T) {
	c := &Conversation{
		Model:    "claude-sonnet-4-6",
		Provider: ProviderAnthropic,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "hi"}}}},
			{Role: RoleAssistant, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "hello"}}}},
		},
	}
	if err := ValidateConversation(c); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateConversation_MissingModel(t *testing.T) {
	c := &Conversation{Provider: ProviderAnthropic}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	assertHasCode(t, err, ErrMissingModel)
}

func TestValidateConversation_MissingProvider(t *testing.T) {
	c := &Conversation{Model: "gpt-4"}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
	assertHasCode(t, err, ErrMissingProvider)
}

func TestValidateConversation_InvalidRole(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{{Role: Role("narrator")}},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	assertHasCode(t, err, ErrInvalidRole)
}

func TestValidateConversation_AlternationViolation_Anthropic(t *testing.T) {
	c := &Conversation{
		Model:    "claude-sonnet-4-6",
		Provider: ProviderAnthropic,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "a"}}}},
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "b"}}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected alternation violation for Anthropic")
	}
	assertHasCode(t, err, ErrAlternationViolation)
}

func TestValidateConversation_NoAlternationCheck_OpenAI(t *testing.T) {
	c := &Conversation{
		Model:    "gpt-4",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "a"}}}},
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "b"}}}},
		},
	}
	err := ValidateConversation(c)
	// OpenAI allows consecutive user messages — should not have alternation violation
	if err != nil {
		assertNotHasCode(t, err, ErrAlternationViolation)
	}
}

func TestValidateConversation_ToolUse_MissingID(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleAssistant, Content: []ContentBlock{{
				Type:    BlockToolUse,
				ToolUse: &ToolUseBlock{Name: "read_file"},
			}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for missing tool use ID")
	}
	assertHasCode(t, err, ErrMissingToolID)
}

func TestValidateConversation_ToolUse_MissingName(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleAssistant, Content: []ContentBlock{{
				Type:    BlockToolUse,
				ToolUse: &ToolUseBlock{ID: "tu_1"},
			}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for missing tool name")
	}
	assertHasCode(t, err, ErrMissingToolName)
}

func TestValidateConversation_OrphanedToolResult(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "go"}}}},
			{Role: RoleAssistant, Content: []ContentBlock{{
				Type:    BlockToolUse,
				ToolUse: &ToolUseBlock{ID: "tu_1", Name: "bash"},
			}}},
			{Role: RoleTool, Content: []ContentBlock{{
				Type:       BlockToolResult,
				ToolResult: &ToolResultBlock{ToolUseID: "tu_999"}, // orphaned
			}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for orphaned tool result")
	}
	assertHasCode(t, err, ErrOrphanedToolResult)
}

func TestValidateConversation_ToolConsistency_Valid(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "go"}}}},
			{Role: RoleAssistant, Content: []ContentBlock{{
				Type:    BlockToolUse,
				ToolUse: &ToolUseBlock{ID: "tu_1", Name: "bash"},
			}}},
			{Role: RoleTool, Content: []ContentBlock{{
				Type:       BlockToolResult,
				ToolResult: &ToolResultBlock{ToolUseID: "tu_1"},
			}}},
		},
	}
	err := ValidateConversation(c)
	if err != nil {
		assertNotHasCode(t, err, ErrOrphanedToolResult)
	}
}

func TestValidateConversation_ThinkingNoSignature(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderAnthropic,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "think"}}}},
			{Role: RoleAssistant, Content: []ContentBlock{{
				Type:     BlockThinking,
				Thinking: &ThinkingBlock{Text: "reasoning here"},
				// No signature — should warn
			}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected warning for thinking without signature")
	}
	assertHasCode(t, err, ErrThinkingNoSignature)
}

func TestValidateConversation_ContentBlock_NoTypedField(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: BlockText}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for content block with no typed field")
	}
	assertHasCode(t, err, ErrInvalidBlockType)
}

func TestValidateConversation_ContentBlock_MultipleTypedFields(t *testing.T) {
	c := &Conversation{
		Model:    "test",
		Provider: ProviderOpenAI,
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{
				Type:  BlockText,
				Text:  &TextBlock{Text: "hi"},
				Image: &ImageBlock{},
			}}},
		},
	}
	err := ValidateConversation(c)
	if err == nil {
		t.Fatal("expected error for multiple typed fields")
	}
	assertHasCode(t, err, ErrInvalidBlockType)
}

// --- ValidateResponse ---

func TestValidateResponse_Valid(t *testing.T) {
	r := &CompletionResponse{
		ID:         "resp_1",
		Model:      "claude-sonnet-4-6",
		StopReason: StopEnd,
		Message:    Message{Role: RoleAssistant, Content: []ContentBlock{{Type: BlockText, Text: &TextBlock{Text: "hi"}}}},
	}
	if err := ValidateResponse(r); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateResponse_MissingID(t *testing.T) {
	r := &CompletionResponse{Model: "test", Message: Message{Role: RoleAssistant}}
	err := ValidateResponse(r)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrEmptyField)
}

func TestValidateResponse_MissingModel(t *testing.T) {
	r := &CompletionResponse{ID: "r1", Message: Message{Role: RoleAssistant}}
	err := ValidateResponse(r)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingModel)
}

func TestValidateResponse_InvalidStopReason(t *testing.T) {
	r := &CompletionResponse{
		ID:         "r1",
		Model:      "test",
		StopReason: StopReason("exploded"),
		Message:    Message{Role: RoleAssistant},
	}
	err := ValidateResponse(r)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrInvalidStopReason)
}

func TestValidateResponse_EmptyStopReason_OK(t *testing.T) {
	r := &CompletionResponse{
		ID:      "r1",
		Model:   "test",
		Message: Message{Role: RoleAssistant},
	}
	err := ValidateResponse(r)
	// Empty stop reason is allowed (streaming)
	if err != nil {
		assertNotHasCode(t, err, ErrInvalidStopReason)
	}
}

func TestValidateResponse_AllStopReasons(t *testing.T) {
	reasons := []StopReason{StopEnd, StopMaxTokens, StopToolUse, StopStopSequence, StopSafety, StopContentFilter, StopRecitation, StopError}
	for _, sr := range reasons {
		t.Run(string(sr), func(t *testing.T) {
			r := &CompletionResponse{
				ID: "r1", Model: "test", StopReason: sr,
				Message: Message{Role: RoleAssistant},
			}
			err := ValidateResponse(r)
			if err != nil {
				assertNotHasCode(t, err, ErrInvalidStopReason)
			}
		})
	}
}

// --- ValidateEvent ---

func TestValidateEvent_Valid(t *testing.T) {
	e := &Event{
		Type:      EventResult,
		Harness:   HarnessClaudeCode,
		SessionID: "sess_1",
		Timestamp: time.Now(),
		Result:    &ResultEvent{Text: "done"},
	}
	if err := ValidateEvent(e); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateEvent_MissingHarness(t *testing.T) {
	e := &Event{Type: EventResult, SessionID: "s1", Timestamp: time.Now(), Result: &ResultEvent{}}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingHarness)
}

func TestValidateEvent_MissingSessionID(t *testing.T) {
	e := &Event{Type: EventResult, Harness: HarnessCodex, Timestamp: time.Now(), Result: &ResultEvent{}}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingSessionID)
}

func TestValidateEvent_MissingTimestamp(t *testing.T) {
	e := &Event{Type: EventResult, Harness: HarnessCodex, SessionID: "s1", Result: &ResultEvent{}}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingTimestamp)
}

func TestValidateEvent_InvalidType(t *testing.T) {
	e := &Event{Type: EventType("explosion"), Harness: HarnessCodex, SessionID: "s1", Timestamp: time.Now()}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrInvalidEventType)
}

func TestValidateEvent_EmptyType(t *testing.T) {
	e := &Event{Harness: HarnessCodex, SessionID: "s1", Timestamp: time.Now()}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrInvalidEventType)
}

func TestValidateEvent_NoPayload(t *testing.T) {
	e := &Event{Type: EventResult, Harness: HarnessCodex, SessionID: "s1", Timestamp: time.Now()}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error for event with no payload")
	}
	assertHasCode(t, err, ErrNoEventPayload)
}

func TestValidateEvent_MultiplePayloads(t *testing.T) {
	e := &Event{
		Type: EventResult, Harness: HarnessCodex, SessionID: "s1", Timestamp: time.Now(),
		Result: &ResultEvent{Text: "a"},
		Error:  &ErrorEvent{Message: "b"},
	}
	err := ValidateEvent(e)
	if err == nil {
		t.Fatal("expected error for multiple payloads")
	}
	assertHasCode(t, err, ErrMultiplePayloads)
}

func TestValidateEvent_AllTypes(t *testing.T) {
	types := []struct {
		et    EventType
		event Event
	}{
		{EventResult, Event{Result: &ResultEvent{Text: "done"}}},
		{EventStream, Event{Stream: &HarnessStream{}}},
		{EventToolCall, Event{ToolCall: &ToolCallEvent{ToolID: "t1", Name: "bash", Input: json.RawMessage(`{}`)}}},
		{EventToolResult, Event{ToolResult: &ToolResultEvent{ToolID: "t1", Name: "bash"}}},
		{EventThinking, Event{Thinking: &ThinkingEvent{Text: "hmm"}}},
		{EventSystem, Event{System: &SystemEvent{Subtype: "init"}}},
		{EventApproval, Event{Approval: &ApprovalEvent{Action: "run", Status: "pending"}}},
		{EventError, Event{Error: &ErrorEvent{Code: "500", Message: "fail"}}},
		{EventSessionState, Event{State: &StateEvent{State: SessionRunning}}},
		{EventPlan, Event{Plan: &PlanEvent{Text: "step 1"}}},
	}
	for _, tt := range types {
		t.Run(string(tt.et), func(t *testing.T) {
			e := tt.event
			e.Type = tt.et
			e.Harness = HarnessClaudeCode
			e.SessionID = "s1"
			e.Timestamp = time.Now()
			if err := ValidateEvent(&e); err != nil {
				t.Errorf("event type %s should be valid, got: %v", tt.et, err)
			}
		})
	}
}

// --- ValidateSession ---

func TestValidateSession_Valid(t *testing.T) {
	s := &Session{ID: "s1", Harness: HarnessClaudeCode, State: SessionIdle}
	if err := ValidateSession(s); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateSession_MissingID(t *testing.T) {
	s := &Session{Harness: HarnessCodex, State: SessionRunning}
	err := ValidateSession(s)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingSessionID)
}

func TestValidateSession_MissingHarness(t *testing.T) {
	s := &Session{ID: "s1", State: SessionRunning}
	err := ValidateSession(s)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrMissingHarness)
}

func TestValidateSession_InvalidState(t *testing.T) {
	s := &Session{ID: "s1", Harness: HarnessCodex, State: SessionState("exploding")}
	err := ValidateSession(s)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrInvalidSessionState)
}

func TestValidateSession_EmptyState(t *testing.T) {
	s := &Session{ID: "s1", Harness: HarnessCodex}
	err := ValidateSession(s)
	if err == nil {
		t.Fatal("expected error")
	}
	assertHasCode(t, err, ErrInvalidSessionState)
}

func TestValidateSession_AllStates(t *testing.T) {
	states := []SessionState{SessionIdle, SessionRunning, SessionCompleted, SessionError, SessionAborted, SessionWaitingApproval}
	for _, st := range states {
		t.Run(string(st), func(t *testing.T) {
			s := &Session{ID: "s1", Harness: HarnessOpenClaw, State: st}
			if err := ValidateSession(s); err != nil {
				t.Errorf("state %s should be valid, got: %v", st, err)
			}
		})
	}
}

// --- ContentBlock.Validate ---

func TestContentBlockValidate_AllBlockTypes(t *testing.T) {
	blocks := []struct {
		name  string
		block ContentBlock
	}{
		{"text", ContentBlock{Type: BlockText, Text: &TextBlock{Text: "hi"}}},
		{"image", ContentBlock{Type: BlockImage, Image: &ImageBlock{}}},
		{"audio", ContentBlock{Type: BlockAudio, Audio: &AudioBlock{}}},
		{"video", ContentBlock{Type: BlockVideo, Video: &VideoBlock{}}},
		{"document", ContentBlock{Type: BlockDocument, Document: &DocumentBlock{}}},
		{"tool_use", ContentBlock{Type: BlockToolUse, ToolUse: &ToolUseBlock{ID: "t1", Name: "bash"}}},
		{"tool_result", ContentBlock{Type: BlockToolResult, ToolResult: &ToolResultBlock{ToolUseID: "t1"}}},
		{"thinking", ContentBlock{Type: BlockThinking, Thinking: &ThinkingBlock{Text: "hmm", Signature: "sig"}}},
		{"redacted_thinking", ContentBlock{Type: BlockRedactedThinking, RedactedThinking: &RedactedThinkingBlock{Data: "x"}}},
		{"code_exec", ContentBlock{Type: BlockCodeExec, CodeExec: &CodeExecBlock{Language: "python", Code: "1+1"}}},
		{"code_exec_result", ContentBlock{Type: BlockCodeExecResult, CodeExecResult: &CodeExecResultBlock{Outcome: "ok", Output: "2"}}},
		{"server_tool_result", ContentBlock{Type: BlockServerToolResult, ServerToolResult: &ServerToolResultBlock{ToolType: "web", Content: json.RawMessage(`{}`)}}},
		{"refusal", ContentBlock{Type: BlockRefusal, Refusal: &RefusalBlock{Text: "no"}}},
	}
	for _, tt := range blocks {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.block.Validate(); err != nil {
				t.Errorf("block type %s should be valid, got: %v", tt.name, err)
			}
		})
	}
}

func TestContentBlockValidate_EmptyType(t *testing.T) {
	cb := ContentBlock{Text: &TextBlock{Text: "hi"}}
	if err := cb.Validate(); err == nil {
		t.Fatal("expected error for empty type")
	}
}

// --- Helpers ---

func assertHasCode(t *testing.T, err *ValidationError, code string) {
	t.Helper()
	for _, f := range err.Failures {
		if f.Code == code {
			return
		}
	}
	t.Errorf("expected failure code %q in: %v", code, err)
}

func assertNotHasCode(t *testing.T, err *ValidationError, code string) {
	t.Helper()
	for _, f := range err.Failures {
		if f.Code == code {
			t.Errorf("unexpected failure code %q in: %v", code, err)
			return
		}
	}
}

// --- ValidationError formatting ---

func TestValidationError_SingleFailure(t *testing.T) {
	e := &ValidationError{Failures: []ValidationFailure{
		{Path: "model", Code: ErrMissingModel, Message: "model is required"},
	}}
	s := e.Error()
	if !strings.Contains(s, "model is required") {
		t.Errorf("unexpected error string: %s", s)
	}
}

func TestValidationError_MultipleFailures(t *testing.T) {
	e := &ValidationError{Failures: []ValidationFailure{
		{Path: "model", Code: ErrMissingModel, Message: "model is required"},
		{Path: "provider", Code: ErrMissingProvider, Message: "provider is required"},
	}}
	s := e.Error()
	if !strings.Contains(s, "2 validation failures") {
		t.Errorf("unexpected error string: %s", s)
	}
}

func TestValidationFailure_StringWithPath(t *testing.T) {
	f := ValidationFailure{Path: "messages[0].role", Code: ErrInvalidRole, Message: "bad role"}
	s := f.String()
	if !strings.Contains(s, "messages[0].role") {
		t.Errorf("path not in string: %s", s)
	}
}

func TestValidationFailure_StringWithoutPath(t *testing.T) {
	f := ValidationFailure{Code: ErrNoEventPayload, Message: "no payload"}
	s := f.String()
	if strings.Contains(s, ":") && strings.HasPrefix(s, ":") {
		t.Errorf("unexpected leading colon: %s", s)
	}
}
