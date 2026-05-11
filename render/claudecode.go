package render

import (
	"errors"

	"github.com/kayushkin/llm-bridge/msg"
)

// claudeCodeRenderer composes CC-specific output from an AgentView.
// Fully implemented in P3. This skeleton establishes the wiring.
type claudeCodeRenderer struct{}

func (claudeCodeRenderer) Harness() msg.Harness { return "claude_code" }

func (claudeCodeRenderer) PreviewBundle(view msg.AgentView) (PreviewBundle, error) {
	return PreviewBundle{}, errors.New("claudecode renderer: not implemented (P3)")
}

func (claudeCodeRenderer) AgentFile(view msg.AgentView) (RenderedFile, error) {
	// CC doesn't materialize per-agent identity files (subagents via --agents
	// JSON, identity via --append-system-prompt). MCP configs and per-agent
	// settings.json are emitted separately via AgentReconciler.EnsureAgent,
	// not via AgentFile.
	return RenderedFile{}, ErrNoFile
}

func (claudeCodeRenderer) SystemPromptAppend(view msg.AgentView) (string, error) {
	return "", errors.New("claudecode renderer: not implemented (P3)")
}

func (claudeCodeRenderer) AgentsJSON(view msg.AgentView) (string, error) {
	return "", errors.New("claudecode renderer: not implemented (P3)")
}

func init() {
	Register(claudeCodeRenderer{})
}
