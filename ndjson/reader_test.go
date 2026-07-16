package ndjson

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

// TestReadLineDeliversLinesFarAboveTheBufferSize is the property both sides of
// the wire depend on: a line much larger than the bufio.Reader's working buffer
// comes back whole. bufio.Scanner could not do this without a cap, and hitting
// that cap looked like EOF.
func TestReadLineDeliversLinesFarAboveTheBufferSize(t *testing.T) {
	const payload = 3 * 1024 * 1024
	wire := strings.Repeat("z", payload) + "\n"

	r := bufio.NewReaderSize(strings.NewReader(wire), 4096)

	line, err := ReadLine(r, MaxLineBytes)
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if len(line) != payload {
		t.Fatalf("got %d bytes, want %d — line was truncated", len(line), payload)
	}
}

// TestReadLineResyncsAfterCeiling pins the memory guard: a line above the
// ceiling is reported as its own error — never as io.EOF — and the reader stays
// framed on line boundaries, so the following line still parses.
func TestReadLineResyncsAfterCeiling(t *testing.T) {
	const limit = 4096

	wire := "first\n" + strings.Repeat("y", limit*3) + "\n" + "third\n"
	r := bufio.NewReaderSize(strings.NewReader(wire), 64)

	line, err := ReadLine(r, limit)
	if err != nil || string(line) != "first" {
		t.Fatalf("line 1: got (%q, %v), want (\"first\", nil)", line, err)
	}

	line, err = ReadLine(r, limit)
	if err != ErrLineTooLong {
		t.Fatalf("line 2: got (%q, %v), want ErrLineTooLong", line, err)
	}
	if line != nil {
		t.Fatalf("line 2: over-ceiling line should be discarded, got %d bytes", len(line))
	}

	line, err = ReadLine(r, limit)
	if err != nil || string(line) != "third" {
		t.Fatalf("line 3: got (%q, %v), want (\"third\", nil) — reader lost newline framing", line, err)
	}
}

// TestReadLineReportsEOFDistinctly pins that end of input is reported as io.EOF
// and nothing else, since io.EOF is the only condition that means "the peer
// exited".
func TestReadLineReportsEOFDistinctly(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader("only\n"), 64)

	if line, err := ReadLine(r, MaxLineBytes); err != nil || string(line) != "only" {
		t.Fatalf("line 1: got (%q, %v), want (\"only\", nil)", line, err)
	}

	line, err := ReadLine(r, MaxLineBytes)
	if err != io.EOF {
		t.Fatalf("at end of input: got err %v, want io.EOF", err)
	}
	if len(line) != 0 {
		t.Fatalf("at end of input: got %q, want no data", line)
	}
}

// TestReadLineReturnsFinalUnterminatedLine covers a peer that dies mid-line:
// the partial data comes back alongside io.EOF rather than being swallowed.
func TestReadLineReturnsFinalUnterminatedLine(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader("truncated"), 64)

	line, err := ReadLine(r, MaxLineBytes)
	if err != io.EOF {
		t.Fatalf("got err %v, want io.EOF", err)
	}
	if string(line) != "truncated" {
		t.Fatalf("got %q, want %q", line, "truncated")
	}
}
