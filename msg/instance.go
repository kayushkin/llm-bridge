package msg

import "time"

// Transport specifies how a harness subprocess is reached on its host.
type Transport string

const (
	TransportLocal  Transport = "local"  // local subprocess on the same host as llm-bridge-server
	TransportSSH    Transport = "ssh"    // SSH from server to remote machine, server forks subprocess via sshd
	TransportRunner Transport = "runner" // remote llm-bridge-runner daemon dialed in over WebSocket
)

// Machine represents a host where harness subprocesses can run. One row per
// physical/virtual host. Multiple instances may share a machine (e.g.
// claude_code and codex on the same laptop). Created either explicitly
// (local/ssh) or auto-created when an llm-bridge-runner enrolls.
type Machine struct {
	ID                string    `json:"id"`                              // m_<id>
	Name              string    `json:"name"`                            // unique, human label: "linode", "wsl-claude"
	Emoji             string    `json:"emoji,omitempty"`                 // optional UI accent: 🏠 🖥 ☁ etc.
	Hostname          string    `json:"hostname,omitempty"`              // network address (Tailscale name, IP, …); empty for runner
	OS                string    `json:"os,omitempty"`                    // GOOS reported by runner, or set on create
	Arch              string    `json:"arch,omitempty"`                  // GOARCH
	Transport         Transport `json:"transport"`
	SSHUser           string    `json:"ssh_user,omitempty"`
	SSHKeyPath        string    `json:"ssh_key_path,omitempty"`
	SSHPort           int       `json:"ssh_port,omitempty"` // default 22
	DefaultWorkingDir string    `json:"default_working_dir,omitempty"`
	User              string    `json:"user,omitempty"`     // runner-side OS user (display only)
	Notes             string    `json:"notes,omitempty"`
	LastSeenAt        time.Time `json:"last_seen_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Instance represents a configured harness deployment on a machine. The
// Machine pointer is populated by API responses for client convenience;
// internal storage code reads from the machines table directly.
type Instance struct {
	ID                    string    `json:"id"`
	HarnessType           Harness   `json:"harness_type"`
	Name                  string    `json:"name"`
	MachineID             string    `json:"machine_id"`
	WorkingDir            string    `json:"working_dir,omitempty"` // overrides Machine.DefaultWorkingDir for this instance
	MaxConcurrentSessions int       `json:"max_concurrent_sessions"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`

	// Machine is populated by API responses (denormalized for the client).
	// Storage and routing code looks up the machine by ID directly.
	Machine *Machine `json:"machine,omitempty"`
}

// InstanceCredential binds a credential to a harness instance with priority.
type InstanceCredential struct {
	InstanceID   string `json:"instance_id"`
	CredentialID string `json:"credential_id"`
	Priority     int    `json:"priority"` // 0 = primary, 1+ = fallbacks
	Enabled      bool   `json:"enabled"`
}

// InstanceStatus aggregates runtime status for an instance.
type InstanceStatus struct {
	Instance       Instance             `json:"instance"`
	ActiveSessions int                  `json:"active_sessions"`
	Credentials    []InstanceCredential `json:"credentials"`
	Reachable      bool                 `json:"reachable"` // SSH/runner connectivity check
	LastChecked    time.Time            `json:"last_checked"`
}

// RunnerEnrollment is a one-time-use token that lets a runner self-register
// a Machine without prior server-side configuration. Minted by an admin
// (CLI or API), redeemed by the runner via POST /api/runner/enroll.
type RunnerEnrollment struct {
	ID                string    `json:"id"`                              // enr_<id>
	ExpiresAt         time.Time `json:"expires_at"`
	UsedAt            time.Time `json:"used_at,omitempty"`               // zero if unused
	ConsumedMachineID string    `json:"consumed_machine_id,omitempty"`   // set when redeemed
	CreatedAt         time.Time `json:"created_at"`
}
