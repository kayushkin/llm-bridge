package render

import (
	"errors"
	"testing"

	"github.com/kayushkin/llm-bridge/msg"
)

func TestRegistry_StubRenderersRegistered(t *testing.T) {
	for _, h := range []msg.Harness{"claude_code", "codex", "inber"} {
		if Get(h) == nil {
			t.Errorf("expected renderer registered for harness %q, got nil", h)
		}
	}
}

func TestClaudeCodeRenderer_AgentFileReturnsErrNoFile(t *testing.T) {
	r := Get("claude_code")
	if r == nil {
		t.Fatal("claude_code renderer not registered")
	}
	_, err := r.AgentFile(msg.AgentView{})
	if !errors.Is(err, ErrNoFile) {
		t.Errorf("expected ErrNoFile, got %v", err)
	}
}

func TestCodexRenderer_AgentsJSONReturnsErrNotSupported(t *testing.T) {
	r := Get("codex")
	if r == nil {
		t.Fatal("codex renderer not registered")
	}
	_, err := r.AgentsJSON(msg.AgentView{})
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestInberRenderer_PreviewBundlePassesThrough(t *testing.T) {
	r := Get("inber")
	if r == nil {
		t.Fatal("inber renderer not registered")
	}
	view := msg.AgentView{
		Agent:   msg.AgentDef{Identity: "I am inber-runtime."},
		Harness: "inber",
	}
	bundle, err := r.PreviewBundle(view)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bundle.Identity != "I am inber-runtime." {
		t.Errorf("expected identity passthrough, got %q", bundle.Identity)
	}
}
