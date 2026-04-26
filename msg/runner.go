package msg

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Runner protocol
//
// The runner protocol is the wire format between llm-bridge-server and an
// llm-bridge-runner daemon deployed on a remote machine. Runners dial the
// server over WebSocket (outbound, NAT-traversal-friendly), authenticate,
// then accept Spawn requests for harness subprocesses. The runner forks the
// requested harness binary locally and pipes its stdio over the same WS
// connection, multiplexed by SessionID.
//
// Layering: the runner is a transparent transport — it does not parse
// harness stdout, it just ships lines verbatim. The server's existing
// NDJSON event pipeline runs unchanged on the received bytes.
// ──────────────────────────────────────────────────────────────────────────────

// RunnerMessageType discriminates the polymorphic RunnerMessage envelope.
type RunnerMessageType string

const (
	// Connection-level (no SessionID).
	RunnerMsgHello   RunnerMessageType = "hello"   // runner → server, sent immediately after connect
	RunnerMsgWelcome RunnerMessageType = "welcome" // server → runner, ack
	RunnerMsgError   RunnerMessageType = "error"   // either direction, fatal error before close
	RunnerMsgPing    RunnerMessageType = "ping"    // app-level keepalive (in addition to WS pings)
	RunnerMsgPong    RunnerMessageType = "pong"

	// Per-session (SessionID required).
	RunnerMsgSpawn  RunnerMessageType = "spawn"  // server → runner
	RunnerMsgStdin  RunnerMessageType = "stdin"  // server → runner
	RunnerMsgSignal RunnerMessageType = "signal" // server → runner
	RunnerMsgStdout RunnerMessageType = "stdout" // runner → server
	RunnerMsgStderr RunnerMessageType = "stderr" // runner → server
	RunnerMsgExit   RunnerMessageType = "exit"   // runner → server
)

// RunnerMessage is the envelope for every frame exchanged on the runner WS.
// Exactly one payload field is populated, matching Type.
type RunnerMessage struct {
	Type      RunnerMessageType `json:"type"`
	SessionID string            `json:"session_id,omitempty"`

	Hello   *RunnerHello   `json:"hello,omitempty"`
	Welcome *RunnerWelcome `json:"welcome,omitempty"`
	Err     *RunnerError   `json:"error,omitempty"`

	Spawn  *RunnerSpawn  `json:"spawn,omitempty"`
	Stdin  *RunnerStdin  `json:"stdin,omitempty"`
	Stdout *RunnerStdout `json:"stdout,omitempty"`
	Stderr *RunnerStderr `json:"stderr,omitempty"`
	Signal *RunnerSignal `json:"signal,omitempty"`
	Exit   *RunnerExit   `json:"exit,omitempty"`
}

// RunnerHello is sent by the runner immediately after the WS upgrade.
// Token authenticates the runner against the server's accepted token list
// (or a per-machine token issued at registration time). The server validates
// it before sending Welcome; on failure it sends RunnerError and closes.
type RunnerHello struct {
	Token              string             `json:"token"`
	MachineName        string             `json:"machine_name"`         // user-chosen label, e.g. "wsl-claude"
	Hostname           string             `json:"hostname"`             // os.Hostname()
	OS                 string             `json:"os"`                   // GOOS
	Arch               string             `json:"arch"`                 // GOARCH
	User               string             `json:"user"`                 // os/user.Current().Username
	WorkingDir         string             `json:"working_dir"`          // default cwd for spawned subprocesses
	AvailableHarnesses []HarnessAvailable `json:"available_harnesses"`  // detected harness binaries on this machine
	RunnerVersion      string             `json:"runner_version"`       // build-stamped version
}

// HarnessAvailable reports a harness binary present on the runner machine.
type HarnessAvailable struct {
	Harness Harness `json:"harness"`
	Binary  string  `json:"binary"`  // absolute path
	Version string  `json:"version,omitempty"`
}

// RunnerWelcome is the server's ack of a successful Hello.
type RunnerWelcome struct {
	MachineID         string    `json:"machine_id"`          // server-assigned, stable across reconnects
	ServerVersion     string    `json:"server_version"`
	PingIntervalSecs  int       `json:"ping_interval_secs"`  // app-level ping cadence the runner should follow
	AcceptedAt        time.Time `json:"accepted_at"`
}

// RunnerError reports a fatal error before the connection is closed.
// Code is a stable machine-readable token; Message is human prose.
type RunnerError struct {
	Code    string `json:"code"`    // "auth_failed", "duplicate_machine", "internal", …
	Message string `json:"message"`
}

// RunnerSpawn asks the runner to fork a harness subprocess for a session.
// StartParams is the same payload the local subprocess transport sends to
// the harness as the "start" JSON-RPC method — the runner writes it
// verbatim to the subprocess's stdin as the first line.
type RunnerSpawn struct {
	Harness     Harness         `json:"harness"`
	BinaryPath  string          `json:"binary_path,omitempty"`  // optional override; runner falls back to PATH lookup
	WorkingDir  string          `json:"working_dir,omitempty"`  // overrides Hello.WorkingDir for this session
	StartParams json.RawMessage `json:"start_params"`           // verbatim JSON-RPC params for "start"
	Env         []string        `json:"env,omitempty"`          // extra env vars, "KEY=VALUE" form
}

// RunnerStdin carries one line (no trailing newline) of NDJSON to be
// written to the subprocess stdin, with a newline appended by the runner.
type RunnerStdin struct {
	Data string `json:"data"`
}

// RunnerStdout carries one line (no trailing newline) read from the
// subprocess stdout. The server's existing pipeline parses it as msg.Event.
type RunnerStdout struct {
	Data string `json:"data"`
}

// RunnerStderr carries one line of subprocess stderr, forwarded for
// server-side logging. Not parsed.
type RunnerStderr struct {
	Data string `json:"data"`
}

// RunnerSignalType is the kind of signal to deliver to a subprocess.
type RunnerSignalType string

const (
	RunnerSignalInterrupt RunnerSignalType = "interrupt" // SIGINT, pause/cancel current turn
	RunnerSignalKill      RunnerSignalType = "kill"      // SIGKILL, terminate immediately
)

// RunnerSignal asks the runner to deliver a signal to a session's subprocess.
type RunnerSignal struct {
	Signal RunnerSignalType `json:"signal"`
}

// RunnerExit reports that a session's subprocess has exited.
type RunnerExit struct {
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"` // non-empty if the process couldn't be reaped cleanly
}
