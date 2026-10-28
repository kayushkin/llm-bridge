package msg

import "strings"

// Codex writes text of its own into a conversation as user messages: the
// working environment, the AGENTS.md instructions it loaded, and markers such
// as an aborted turn. In a rollout file these are `response_item` messages
// with role "user", indistinguishable by role from what the person typed. A
// discovered session titled from the first of them shows the instructions
// instead of the request, and a turn count that includes them is wrong.
//
// codexInjectedTextPrefixes lists how each such message begins. The harness
// wrapper reads it to skip them when it takes a session's prompt, and
// llm-bridge-server reads it to recognise a stored title that came from one,
// so the two never disagree about what counts as Codex's own text.
var codexInjectedTextPrefixes = []string{
	"<environment_context>",
	"# AGENTS.md instructions",
	"<INSTRUCTIONS>",
	"<user_instructions>",
	"<turn_aborted>",
	"<recommended_plugins",
}

// CodexInjectedTextPrefixes returns a copy of how each message Codex injects
// begins, for a reader that must recognise them where it cannot call
// IsCodexInjectedText on each text: log-store builds its search's exclusion
// from it, so a search does not match the instructions Codex loaded.
func CodexInjectedTextPrefixes() []string {
	return append([]string(nil), codexInjectedTextPrefixes...)
}

// IsCodexInjectedText reports whether a user-role message, or a title taken
// from one, is text Codex injected rather than something a person wrote.
// Leading whitespace is ignored.
func IsCodexInjectedText(text string) bool {
	trimmed := strings.TrimLeft(text, " \t\r\n")
	for _, prefix := range codexInjectedTextPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
