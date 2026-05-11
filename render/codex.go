package render

import (
	"errors"

	"github.com/kayushkin/llm-bridge/msg"
)

// codexRenderer composes Codex-specific output. Fully implemented in P17
// (Codex catalog audit + wrapper). This skeleton establishes the wiring.
type codexRenderer struct{}

func (codexRenderer) Harness() msg.Harness { return "codex" }

func (codexRenderer) PreviewBundle(view msg.AgentView) (PreviewBundle, error) {
	return PreviewBundle{}, errors.New("codex renderer: not implemented (P17)")
}

func (codexRenderer) AgentFile(view msg.AgentView) (RenderedFile, error) {
	// Codex doesn't materialize a per-agent file; identity goes via
	// codex app-server initialize. AGENTS.md (if any) is treated as
	// user-authored project conventions, not a per-agent artifact.
	return RenderedFile{}, ErrNoFile
}

func (codexRenderer) SystemPromptAppend(view msg.AgentView) (string, error) {
	return "", errors.New("codex renderer: not implemented (P17)")
}

func (codexRenderer) AgentsJSON(view msg.AgentView) (string, error) {
	// Codex has no native equivalent to CC's --agents inline subagents.
	// Subagents are delegated via `bridge agent ask` CLI-in-prompt instead.
	return "", ErrNotSupported
}

func init() {
	Register(codexRenderer{})
}
