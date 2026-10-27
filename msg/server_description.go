package msg

import "time"

// ServerNameHeader carries the sending server's own name (its server.name
// setting) on every request and answer one llm-bridge-server sends another
// over a peer connection, so each side always shows the name the other gives
// itself. An empty value means the sender has no name; an absent header means
// a sender too old to say.
const ServerNameHeader = "X-LLM-Bridge-Server-Name"

// ServerDescription answers GET /server: what this llm-bridge-server calls
// itself, and whether it is holding the machine awake.
type ServerDescription struct {
	// Name is the server.name setting. Empty when it is not set; a server
	// with no name says so rather than inventing one.
	Name      string          `json:"name"`
	KeepAwake ServerKeepAwake `json:"keep_awake"`
}

// ServerKeepAwake is the keep_awake settings and what they are doing now.
type ServerKeepAwake struct {
	// Enabled is the keep_awake.enabled setting.
	Enabled bool `json:"enabled"`
	// Held is set while this server holds a sleep block on its machine.
	Held bool `json:"held"`
	// Until is when the block ends if nothing else happens: the last activity
	// (a session running, or a message sent to one) plus the grace period.
	// It moves forward while sessions keep running. Empty when no block is
	// held.
	Until *time.Time `json:"until,omitempty"`
	// Error is why the last attempt to hold a block failed, empty when it did
	// not.
	Error string `json:"error,omitempty"`
}
