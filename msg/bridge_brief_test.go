package msg

import "testing"

func TestBridgeBriefText(t *testing.T) {
	brief := BridgeBrief{
		Kind:  BridgeBriefKindPeerRequest,
		From:  "llm-bridge-server",
		Title: `Fix "the" <thing> & more`,
		Fence: "0011223344556677",
		Body: "- **Asked by:** laptop\n\n" +
			BridgeBriefQuoteStart("request", "0011223344556677", ": two\nlines") + "\nhello\n" +
			BridgeBriefQuoteEnd("request", "0011223344556677") + "\n\n",
	}
	want := `<bridge-brief kind="peer-request" from="llm-bridge-server" title="Fix &quot;the&quot; &lt;thing&gt; &amp; more" fence="0011223344556677">
- **Asked by:** laptop

--- request 0011223344556677: two lines ---
hello
--- end of request 0011223344556677 ---
</bridge-brief>`
	if got := brief.Text(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestBridgeBriefWithoutFenceNamesNone(t *testing.T) {
	got := BridgeBrief{Kind: BridgeBriefKindSweep, From: "orchestratord", Title: "Sweep", Body: "body"}.Text()
	want := "<bridge-brief kind=\"sweep\" from=\"orchestratord\" title=\"Sweep\">\nbody\n</bridge-brief>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
