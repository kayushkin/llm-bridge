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

	// Seeding (no SessionID; carry context/skill files from the bridge to the runner).
	RunnerMsgSeedDelta    RunnerMessageType = "seed_delta"    // server → runner, single-file delta (push)
	RunnerMsgSeedSnapshot RunnerMessageType = "seed_snapshot" // server → runner, full manifest snapshot (after scan)
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

	SeedDelta    *RunnerSeedDelta    `json:"seed_delta,omitempty"`
	SeedSnapshot *RunnerSeedSnapshot `json:"seed_snapshot,omitempty"`
}

// RunnerHello is sent by the runner immediately after the WS upgrade.
// Authentication is the Authorization: Bearer header on the upgrade
// request — the server resolves the token to a Machine row by hash.
// The Hello payload is environmental detail the server records for
// display + binary-serving (hostname, OS, etc.); it carries no identity
// claims of its own. The machine label is server-side state, returned
// to the runner via Welcome.MachineName.
type RunnerHello struct {
	Hostname           string             `json:"hostname"`            // os.Hostname()
	OS                 string             `json:"os"`                  // GOOS
	Arch               string             `json:"arch"`                // GOARCH
	User               string             `json:"user"`                // os/user.Current().Username
	WorkingDir         string             `json:"working_dir"`         // default cwd for spawned subprocesses
	AvailableHarnesses []HarnessAvailable `json:"available_harnesses"` // detected harness binaries on this machine
	RunnerVersion      string             `json:"runner_version"`      // build-stamped version
}

// HarnessAvailable reports a harness binary present on the runner machine.
type HarnessAvailable struct {
	Harness Harness `json:"harness"`
	Binary  string  `json:"binary"`  // absolute path
	Version string  `json:"version,omitempty"`
}

// RunnerWelcome is the server's ack of a successful Hello.
//
// MachineName is the canonical name on the server side. Runners with a
// stale local label (e.g. machine renamed in the UI after enrollment)
// should overwrite their saved config from this value so subsequent
// connects don't keep reporting the old name.
type RunnerWelcome struct {
	MachineID        string    `json:"machine_id"`         // server-assigned, stable across reconnects
	MachineName      string    `json:"machine_name"`       // authoritative current name
	ServerVersion    string    `json:"server_version"`
	PingIntervalSecs int       `json:"ping_interval_secs"` // app-level ping cadence the runner should follow
	AcceptedAt       time.Time `json:"accepted_at"`
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
//
// Provision lists long-lived backend services the wrapper expects to be
// running on the same host (e.g. inber-server for the inber wrapper).
// The runner ensures each is installed and running before the fork —
// idempotently, so subsequent spawns don't restart a healthy backend.
// This is how "create an instance on a remote machine" actually
// deploys the harness's full stack to that machine.
type RunnerSpawn struct {
	Harness     Harness           `json:"harness"`
	BinaryPath  string            `json:"binary_path,omitempty"`  // optional override; runner falls back to PATH lookup
	WorkingDir  string            `json:"working_dir,omitempty"`  // overrides Hello.WorkingDir for this session
	StartParams json.RawMessage   `json:"start_params"`           // verbatim JSON-RPC params for "start"
	Env         []string          `json:"env,omitempty"`          // extra env vars for the wrapper, "KEY=VALUE" form
	Provision   []HarnessService  `json:"provision,omitempty"`    // backend services the runner should ensure are running
}

// HarnessService describes one long-lived backend daemon a harness needs
// running on the runner host. The bridge populates this from its own
// per-harness deployment manifest, with credential values already
// resolved into Env so the runner doesn't need bridge-side state.
type HarnessService struct {
	Name      string   `json:"name"`                  // unique key, e.g. "inber-server"; runners dedupe by this across spawns
	BinaryURL string   `json:"binary_url"`            // canonical /api/runner/binary?name=…&os=…&arch=… URL
	BinaryPath string  `json:"binary_path,omitempty"` // absolute path on the runner; defaults to ~/.local/bin/<name>
	Args      []string `json:"args,omitempty"`        // command-line args passed to the binary
	Env       []string `json:"env,omitempty"`         // env vars (KEY=VALUE), including resolved credential values
	HealthURL string   `json:"health_url"`            // GET probe to wait for after start (e.g. http://localhost:8200/health)
	Stdout    string   `json:"stdout,omitempty"`      // optional log file path; defaults to ~/.cache/llm-bridge-runner/<name>.log
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

// SeedSource discriminates which store provided a seeded file. Lets the
// runner reconcile each provider against its own endpoint set without the
// bridge needing per-source message types.
type SeedSource string

const (
	SeedSourceAgentStore SeedSource = "agent-store" // CLAUDE.md, AGENTS.md, memory, subagents, …
	SeedSourceSkillStore SeedSource = "skill-store" // ~/.claude/skills/**
)

// SeedOp is the kind of change the runner should apply to a seeded path.
type SeedOp string

const (
	SeedOpUpsert SeedOp = "upsert" // create or overwrite (drift-checked)
	SeedOpDelete SeedOp = "delete" // remove (only if locally still equals last-pushed sha)
)

// RunnerSeedDelta is a single-file change pushed to a runner immediately
// after the bridge records a new authoritative version. The runner must:
//
//  1. Compute the sha256 of the local file at InstallPath, if any.
//  2. If the local sha differs from ExpectedPriorSHA (drift), POST the local
//     bytes to the source's /seed/drift endpoint *first*. Only if that call
//     succeeds may it proceed to step 3.
//  3. GET the new content from the source's version endpoint and write it
//     atomically (tmp + rename).
//
// ExpectedPriorSHA empty = the runner has never seen this file (first push).
// In that case any pre-existing content at InstallPath is treated as drift.
type RunnerSeedDelta struct {
	Source        SeedSource `json:"source"`
	Op            SeedOp     `json:"op"`
	TrackedID     int64      `json:"tracked_id"`             // ID in the source store
	VersionID     int64      `json:"version_id,omitempty"`   // version to fetch on upsert
	InstallPath   string     `json:"install_path"`           // $HOME-relative
	NewSHA        string     `json:"new_sha,omitempty"`      // sha of the new content (upsert only)
	ExpectedPriorSHA string  `json:"expected_prior_sha,omitempty"` // last-pushed sha bridge thinks runner has
}

// RunnerSeedSnapshot tells the runner to do a full manifest reconcile against
// the named source. Sent on connect and after a bridge-side scan completes.
// The actual manifest is fetched via HTTP — this message is just the trigger
// + opportunity to bundle metadata (e.g. ticket count) for telemetry.
type RunnerSeedSnapshot struct {
	Source     SeedSource `json:"source"`
	EntryCount int        `json:"entry_count,omitempty"` // bridge's count, for sanity logging
	Reason     string     `json:"reason,omitempty"`      // "connect", "scan", "manual", …
}
