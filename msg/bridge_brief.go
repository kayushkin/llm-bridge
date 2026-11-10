package msg

import (
	"fmt"
	"strings"
)

// A bridge brief is a message a service writes and sends into a session as
// its user message: the first message of a session started from a connected
// server's patch or request, the orchestrator's request to place one, the
// orchestrator's sweep. The agent reads all of it; a chat client draws it as
// a card rather than as something the person typed.
//
// Its whole text is one block:
//
//	<bridge-brief kind="peer-patch" from="llm-bridge-server" title="Land a patch to reference-store" fence="a1f5667967b81d76">
//	markdown written by the service
//	--- summary a1f5667967b81d76 from its-skayushkin ---
//	text the service quotes from someone else
//	--- end of summary a1f5667967b81d76 ---
//	</bridge-brief>
//
// The body is markdown. When the brief quotes text it did not write, it names
// a fence: a random tag the quoting service drew, so the quoted text cannot
// close its own quote. The quote starts at the first line
// "--- <noun> <fence><caption> ---" and ends at the first line after it that
// reads "--- end of <noun> <fence> ---". A brief quotes at most once; the
// quoted text may itself be a whole bridge brief.
const (
	BridgeBriefKindPeerPatch     = "peer-patch"
	BridgeBriefKindPeerRequest   = "peer-request"
	BridgeBriefKindPeerPlacement = "peer-placement"
	BridgeBriefKindSweep         = "sweep"
)

// BridgeBrief is one brief before it is written out.
type BridgeBrief struct {
	// Kind is one of the BridgeBriefKind constants.
	Kind string
	// From names the service that wrote the brief, such as llm-bridge-server.
	From string
	// Title is one line a client shows as the card's heading.
	Title string
	// Fence is the quote's tag, empty when the brief quotes nothing.
	Fence string
	// Body is the markdown, quote included.
	Body string
}

// Text is the brief as a session receives it.
func (brief BridgeBrief) Text() string {
	var b strings.Builder
	b.WriteString("<bridge-brief")
	writeBridgeBriefAttribute(&b, "kind", brief.Kind)
	writeBridgeBriefAttribute(&b, "from", brief.From)
	writeBridgeBriefAttribute(&b, "title", brief.Title)
	if brief.Fence != "" {
		writeBridgeBriefAttribute(&b, "fence", brief.Fence)
	}
	b.WriteString(">\n")
	b.WriteString(strings.TrimRight(brief.Body, "\n"))
	b.WriteString("\n</bridge-brief>")
	return b.String()
}

// BridgeBriefQuoteStart is the line that opens a brief's quote, such as
// "--- summary <fence> from its-skayushkin ---" for noun "summary" and
// caption " from its-skayushkin". The noun is one word.
func BridgeBriefQuoteStart(noun, fence, caption string) string {
	return fmt.Sprintf("--- %s %s%s ---", noun, fence, oneBridgeBriefLine(caption))
}

// BridgeBriefQuoteEnd is the line that closes it.
func BridgeBriefQuoteEnd(noun, fence string) string {
	return fmt.Sprintf("--- end of %s %s ---", noun, fence)
}

var bridgeBriefAttributeEscaper = strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;", "\n", " ", "\r", " ")

func writeBridgeBriefAttribute(b *strings.Builder, name, value string) {
	b.WriteString(" " + name + `="` + bridgeBriefAttributeEscaper.Replace(value) + `"`)
}

func oneBridgeBriefLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
