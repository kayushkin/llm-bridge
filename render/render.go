// Package render produces harness-specific output (CLI flag values, MCP
// configs, system-prompt fragments) from canonical AgentView definitions.
//
// Renderers are PURE FUNCTIONS — same input bytes, same output bytes. Used
// by both agent-store (for the /files preview tab) and each
// llm-bridge-<harness> wrapper (in AgentReconciler.EnsureAgent and
// PrepareSession). See ~/repos/llm-bridge-server/AGENT-MANAGEMENT.md.
package render

import (
	"errors"
	"fmt"

	"github.com/kayushkin/llm-bridge/msg"
)

// Renderer converts a canonical AgentView into harness-specific bytes.
// Implementations are stateless and pure.
type Renderer interface {
	// Harness returns the harness this renderer targets.
	Harness() msg.Harness

	// PreviewBundle returns the assembled context the harness would receive
	// at session start, broken into sections for inspection. Used by
	// agent-store's /files preview tab.
	PreviewBundle(view msg.AgentView) (PreviewBundle, error)

	// AgentFile returns the file (if any) that should be materialized for
	// this agent on the harness side. For harnesses with no per-agent file
	// (e.g. Claude Code, which uses --agents JSON inline), returns
	// (RenderedFile{}, ErrNoFile).
	AgentFile(view msg.AgentView) (RenderedFile, error)

	// SystemPromptAppend returns the text to pass via the harness's
	// "append-to-system-prompt" mechanism (CC --append-system-prompt,
	// Codex JSON-RPC field, etc.). May be empty.
	SystemPromptAppend(view msg.AgentView) (string, error)

	// AgentsJSON returns the inline subagent JSON map for harnesses that
	// support it (CC --agents). Returns ("", ErrNotSupported) on harnesses
	// without an equivalent mechanism.
	AgentsJSON(view msg.AgentView) (string, error)
}

// PreviewBundle is the inspection projection of what a renderer would emit.
// Used by /files preview to show the human what the harness will see.
type PreviewBundle struct {
	Harness            msg.Harness    `json:"harness"`
	Identity           string         `json:"identity,omitempty"`
	SystemPromptAppend string         `json:"system_prompt_append,omitempty"`
	AgentsJSON         string         `json:"agents_json,omitempty"`
	AllowedTools       []string       `json:"allowed_tools,omitempty"`
	Files              []RenderedFile `json:"files,omitempty"`
	BundleSHA256       string         `json:"bundle_sha256"`
}

// RenderedFile is a materialized file output of a renderer. The renderer
// returns the bytes and target path; the caller writes the file.
type RenderedFile struct {
	Path   string `json:"path"`             // absolute path
	Body   []byte `json:"body"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode,omitempty"`   // default 0644 if zero
}

// ErrNoFile signals that a renderer has nothing to materialize for this
// harness/agent combination. Common for Claude Code subagents (passed via
// --agents JSON), inber-runtime agents (state lives in agent-store), etc.
var ErrNoFile = errors.New("render: no file to materialize for this harness")

// ErrNotSupported signals a renderer feature that this harness does not
// expose (e.g. AgentsJSON on non-CC harnesses).
var ErrNotSupported = errors.New("render: feature not supported by this harness")

// Registry maps harness IDs to their renderers. Each llm-bridge-<harness>
// wrapper registers its renderer via Register during init().
var Registry = map[msg.Harness]Renderer{}

// Register adds a renderer to the global registry. Typically called from
// the renderer package's init() function. Panics on duplicate registration
// since this is a wiring error caught at startup.
func Register(r Renderer) {
	h := r.Harness()
	if _, exists := Registry[h]; exists {
		panic(fmt.Sprintf("render: renderer for harness %q already registered", h))
	}
	Registry[h] = r
}

// Get returns the renderer for the named harness, or nil if none is
// registered. Callers should handle the nil case explicitly.
func Get(h msg.Harness) Renderer {
	return Registry[h]
}
