package msg

import "testing"

func TestIsCodexInjectedTextRecognisesWhatCodexWritesAsUserMessages(t *testing.T) {
	injected := []string{
		"<environment_context>\n  <cwd>/work</cwd>\n</environment_context>",
		"# AGENTS.md instructions for /work\n\n<INSTRUCTIONS>\n# Directives",
		"<INSTRUCTIONS>\n# Directives",
		"<user_instructions>\nbe brief\n</user_instructions>",
		"<turn_aborted>\nThe user interrupted the previous turn.\n</turn_aborted>",
		"<recommended_plugins>…",
		"\n\n  # AGENTS.md instructions for /work",
	}
	for _, text := range injected {
		if !IsCodexInjectedText(text) {
			t.Errorf("IsCodexInjectedText(%q) = false, want true", text)
		}
	}
	written := []string{
		"Which are the recent sessions?",
		"Read AGENTS.md and tell me what it says",
		"# A heading the person typed",
		"<b>bold</b> is how you write it",
		"",
	}
	for _, text := range written {
		if IsCodexInjectedText(text) {
			t.Errorf("IsCodexInjectedText(%q) = true, want false", text)
		}
	}
}
