package msg

import (
	"encoding/json"
	"fmt"
)

// BlockType identifies the kind of content block.
type BlockType string

const (
	BlockText             BlockType = "text"
	BlockImage            BlockType = "image"
	BlockAudio            BlockType = "audio"
	BlockVideo            BlockType = "video"
	BlockDocument         BlockType = "document"
	BlockToolUse          BlockType = "tool_use"
	BlockToolResult       BlockType = "tool_result"
	BlockThinking         BlockType = "thinking"
	BlockRedactedThinking BlockType = "redacted_thinking"
	BlockCodeExec         BlockType = "code_exec"
	BlockCodeExecResult   BlockType = "code_exec_result"
	BlockServerToolResult BlockType = "server_tool_result"
	BlockRefusal          BlockType = "refusal"
)

// ContentBlock is a discriminated union of all possible content block types.
// Exactly one of the typed fields will be non-nil, matching Type.
type ContentBlock struct {
	Type BlockType `json:"type"`

	Text             *TextBlock             `json:"text_block,omitempty"`
	Image            *ImageBlock            `json:"image_block,omitempty"`
	Audio            *AudioBlock            `json:"audio_block,omitempty"`
	Video            *VideoBlock            `json:"video_block,omitempty"`
	Document         *DocumentBlock         `json:"document_block,omitempty"`
	ToolUse          *ToolUseBlock          `json:"tool_use_block,omitempty"`
	ToolResult       *ToolResultBlock       `json:"tool_result_block,omitempty"`
	Thinking         *ThinkingBlock         `json:"thinking_block,omitempty"`
	RedactedThinking *RedactedThinkingBlock `json:"redacted_thinking_block,omitempty"`
	CodeExec         *CodeExecBlock         `json:"code_exec_block,omitempty"`
	CodeExecResult   *CodeExecResultBlock   `json:"code_exec_result_block,omitempty"`
	ServerToolResult *ServerToolResultBlock `json:"server_tool_result_block,omitempty"`
	Refusal          *RefusalBlock          `json:"refusal_block,omitempty"`

	// Extensions holds bridge-specific typed data not yet in the canonical schema.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// Validate checks that exactly one typed field is set and matches Type.
func (cb *ContentBlock) Validate() error {
	if cb.Type == "" {
		return fmt.Errorf("content block: type is empty")
	}
	set := 0
	if cb.Text != nil {
		set++
	}
	if cb.Image != nil {
		set++
	}
	if cb.Audio != nil {
		set++
	}
	if cb.Video != nil {
		set++
	}
	if cb.Document != nil {
		set++
	}
	if cb.ToolUse != nil {
		set++
	}
	if cb.ToolResult != nil {
		set++
	}
	if cb.Thinking != nil {
		set++
	}
	if cb.RedactedThinking != nil {
		set++
	}
	if cb.CodeExec != nil {
		set++
	}
	if cb.CodeExecResult != nil {
		set++
	}
	if cb.ServerToolResult != nil {
		set++
	}
	if cb.Refusal != nil {
		set++
	}
	if set == 0 {
		return fmt.Errorf("content block type %q: no typed field set", cb.Type)
	}
	if set > 1 {
		return fmt.Errorf("content block type %q: %d typed fields set, expected 1", cb.Type, set)
	}
	return nil
}

// --- Concrete block types ---

// TextBlock represents plain text content.
type TextBlock struct {
	Text         string        `json:"text"`
	Citations    []Citation    `json:"citations,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// Citation references a source document by character position.
type Citation struct {
	Type           string `json:"type"`
	CitedText      string `json:"cited_text"`
	DocumentIndex  int    `json:"document_index"`
	StartCharIndex int    `json:"start_char_index,omitempty"`
	EndCharIndex   int    `json:"end_char_index,omitempty"`
	StartPage      int    `json:"start_page,omitempty"`
	EndPage        int    `json:"end_page,omitempty"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// CacheControl is Anthropic's per-block cache hint.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
	TTL  string `json:"ttl,omitempty"`
}

// MediaSourceKind identifies how media data is provided.
type MediaSourceKind string

const (
	MediaBase64 MediaSourceKind = "base64"
	MediaURL    MediaSourceKind = "url"
	MediaFileID MediaSourceKind = "file_id"
)

// MediaSource unifies how media (images, audio, video, docs) are referenced.
type MediaSource struct {
	Kind      MediaSourceKind `json:"kind"`
	MediaType string          `json:"media_type"`
	Data      string          `json:"data"`
	Detail    string          `json:"detail,omitempty"`
	Format    string          `json:"format,omitempty"`
	Filename  string          `json:"filename,omitempty"`
}

// ImageBlock represents an image in any encoding.
type ImageBlock struct {
	Source       MediaSource   `json:"source"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// AudioBlock represents audio content.
type AudioBlock struct {
	Source       MediaSource   `json:"source"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// VideoBlock represents video content.
type VideoBlock struct {
	Source       MediaSource   `json:"source"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// DocumentBlock represents a document (PDF, plain text, etc.).
type DocumentBlock struct {
	Source           MediaSource   `json:"source"`
	Title            string        `json:"title,omitempty"`
	CitationsEnabled bool          `json:"citations_enabled,omitempty"`
	CacheControl     *CacheControl `json:"cache_control,omitempty"`
}

// ToolUseBlock represents the model requesting a tool call.
type ToolUseBlock struct {
	ID           string          `json:"id"`
	OriginalID   string          `json:"original_id"`
	Name         string          `json:"name"`
	Input        json.RawMessage `json:"input"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

// ToolResultBlock represents the result of a tool call.
type ToolResultBlock struct {
	ToolUseID         string         `json:"tool_use_id"`
	OriginalToolUseID string         `json:"original_tool_use_id"`
	Content           []ContentBlock `json:"content,omitempty"`
	IsError           bool           `json:"is_error,omitempty"`
	SourceRole        string         `json:"source_role,omitempty"`
	ToolName          string         `json:"tool_name,omitempty"`
}

// ThinkingBlock represents model reasoning/thinking traces.
type ThinkingBlock struct {
	Text         string `json:"text"`
	Signature    string `json:"signature,omitempty"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// RedactedThinkingBlock represents thinking content redacted by the provider.
type RedactedThinkingBlock struct {
	Data string `json:"data"`
}

// CodeExecBlock represents a code execution request.
type CodeExecBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// CodeExecResultBlock represents the output of code execution.
type CodeExecResultBlock struct {
	Outcome string `json:"outcome"`
	Output  string `json:"output"`
}

// ServerToolResultBlock represents results from provider-hosted tools.
type ServerToolResultBlock struct {
	ToolType string          `json:"tool_type"`
	Content  json.RawMessage `json:"content"`

	Overflow map[string]json.RawMessage `json:"_overflow,omitempty"`
}

// RefusalBlock represents the model explicitly refusing a request.
type RefusalBlock struct {
	Text string `json:"text"`
}
