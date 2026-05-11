package render

import (
	"github.com/kayushkin/llm-bridge/msg"
)

// inberRenderer is a near-passthrough — inber-runtime sessions read agent
// state directly from agent-store; no file materialization needed. The
// renderer exists for the /files preview tab so inber agents show up
// alongside others, and to satisfy the registry pattern.
type inberRenderer struct{}

func (inberRenderer) Harness() msg.Harness { return "inber" }

func (inberRenderer) PreviewBundle(view msg.AgentView) (PreviewBundle, error) {
	identity := view.Agent.Identity
	if view.Agent.HarnessConfig != nil && view.Agent.HarnessConfig.SystemPrompt != "" {
		identity = view.Agent.HarnessConfig.SystemPrompt
	}
	return PreviewBundle{
		Harness:  view.Harness,
		Identity: identity,
	}, nil
}

func (inberRenderer) AgentFile(view msg.AgentView) (RenderedFile, error) {
	return RenderedFile{}, ErrNoFile
}

func (inberRenderer) SystemPromptAppend(view msg.AgentView) (string, error) {
	if view.Agent.HarnessConfig != nil && view.Agent.HarnessConfig.SystemPrompt != "" {
		return view.Agent.HarnessConfig.SystemPrompt, nil
	}
	return view.Agent.Identity, nil
}

func (inberRenderer) AgentsJSON(view msg.AgentView) (string, error) {
	return "", ErrNotSupported
}

func init() {
	Register(inberRenderer{})
}
