// Package ndjson reads the newline-delimited JSON stream that carries JSON-RPC
// across the harness wire — the requests the gateway writes on a harness's
// stdin and the events a harness writes back on its stdout.
//
// It exists because bufio.Scanner is the wrong tool for this wire. Scanner
// enforces a maximum token size and signals an overlong line by ending the
// scan — which at the call site is indistinguishable from EOF. Both sides read
// the peer ending its stream as the peer exiting: a harness whose stdin scanner
// hits its cap logs "stdin closed, shutting down" and kills the session while
// the gateway is still alive and still writing, and the gateway's stdout reader
// closes every SSE subscriber and zeroes the PID while the harness is still
// streaming.
//
// A single oversized line is enough to trigger that. The gateway marshals a
// whole message — including base64 image blocks for multimodal turns — onto one
// stdin line, and harness bridges pass multi-megabyte events (a Playwright
// screenshot, a large file read, a long tool result) back unchanged. ReadLine
// therefore imposes no practical length limit, and reports the one case it does
// refuse as its own error, distinct from io.EOF.
package ndjson

import (
	"bufio"
	"errors"
)

// MaxLineBytes is the ceiling on a single NDJSON line. It is not a wire limit
// — it sits far above any real request or event — but a guard so a runaway peer
// that never emits a newline cannot grow the reader's heap without bound.
const MaxLineBytes = 64 * 1024 * 1024

// ErrLineTooLong reports a line above the caller's limit. The offending line is
// discarded and the reader resynchronizes on the next newline, so a caller that
// logs this and continues keeps the session alive and correctly framed.
var ErrLineTooLong = errors.New("ndjson: line exceeds the size ceiling")

// ReadLine reads one newline-terminated line from r and returns it without the
// trailing newline. Lines are bounded only by limit.
//
// The returned error is nil for a complete line, io.EOF at end of input (with
// any final unterminated line returned alongside it), ErrLineTooLong for a line
// above limit, or the underlying reader's error. ErrLineTooLong and io.EOF are
// deliberately distinct: only io.EOF means the peer is gone.
func ReadLine(r *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	overlong := false

	for {
		chunk, err := r.ReadSlice('\n')

		if !overlong {
			if len(line)+len(chunk) > limit {
				// Stop accumulating, but keep draining to the next
				// newline so the reader stays framed on line
				// boundaries and the following lines still parse.
				overlong = true
				line = nil
			} else {
				line = append(line, chunk...)
			}
		}

		switch err {
		case nil:
			if overlong {
				return nil, ErrLineTooLong
			}
			return line[:len(line)-1], nil // drop the '\n'
		case bufio.ErrBufferFull:
			continue // partial line — keep reading
		default:
			if overlong {
				return nil, ErrLineTooLong
			}
			return line, err
		}
	}
}
