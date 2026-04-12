// Package bridge defines the interfaces that provider and harness bridges implement.
//
// API bridges (anthropic, openai, gemini, openrouter) are stateless converters
// between the canonical msg types and provider-specific wire formats.
//
// Harness bridges (claudecode, codex, openclaw) manage session lifecycle —
// subprocesses, WebSocket connections, file tailing — and emit msg.Event streams.
package bridge

import (
	"context"
	"encoding/json"
	"io"

	"github.com/kayushkin/llm-bridge/msg"
)

// APIBridge converts between canonical msg types and a provider's wire format.
// Implementations are stateless — they don't make HTTP calls or manage connections.
// The caller (gateway or consumer code) handles transport.
type APIBridge interface {
	// Provider returns which provider this bridge handles.
	Provider() msg.Provider

	// BuildRequest converts a canonical Conversation into provider-specific JSON.
	BuildRequest(conv *msg.Conversation) ([]byte, error)

	// ParseResponse converts provider-specific JSON into a canonical CompletionResponse.
	// The raw JSON is preserved in CompletionResponse.Raw.
	ParseResponse(data []byte) (*msg.CompletionResponse, error)

	// ParseStreamEvent converts a single provider-specific stream event into
	// a canonical StreamEvent. Returns nil for events that have no canonical mapping
	// (e.g. provider-specific keepalives).
	ParseStreamEvent(data []byte) (*msg.StreamEvent, error)
}

// HarnessBridge manages an agent harness lifecycle and emits canonical events.
// Implementations own their transport (subprocess, WebSocket, file tailing).
type HarnessBridge interface {
	// Harness returns which harness this bridge handles.
	Harness() msg.Harness

	// Start begins a new session with the given prompt and config.
	Start(ctx context.Context, prompt string, config json.RawMessage) (HarnessSession, error)

	// Resume continues an existing session.
	Resume(ctx context.Context, sessionID string, prompt string, config json.RawMessage) (HarnessSession, error)
}

// HarnessSession is a running or resumable harness session.
type HarnessSession interface {
	// ID returns the session identifier.
	ID() string

	// Events returns a channel of canonical events from the session.
	// The channel is closed when the session ends (completion, error, or cancellation).
	Events() <-chan msg.Event

	// Stop gracefully interrupts the session. The session may be resumable after stopping.
	Stop() error
}

// StreamReader is a convenience interface for reading provider SSE/NDJSON streams.
// API bridges can optionally implement this for streaming support.
type StreamReader interface {
	// ReadStream reads a provider's streaming response and emits canonical StreamEvents.
	// The reader contains the raw SSE or NDJSON stream body.
	ReadStream(r io.Reader) <-chan msg.StreamEvent
}
