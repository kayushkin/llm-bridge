package msg

import "time"

// Transport specifies how to connect to a harness instance.
type Transport string

const (
	TransportLocal Transport = "local" // local subprocess
	TransportSSH   Transport = "ssh"   // SSH to remote machine
)

// Instance represents a deployed harness on a specific machine.
// A harness type (claudecode, codex) is a template; an instance is a running deployment.
type Instance struct {
	ID                    string    `json:"id"`
	HarnessType           Harness   `json:"harness_type"`
	Name                  string    `json:"name"` // human label: "laptop-cc", "ci-runner-1"
	Host                  string    `json:"host"` // "localhost", "dev.internal", "ci-01.example.com"
	Transport             Transport `json:"transport"`
	SSHUser               string    `json:"ssh_user,omitempty"`
	SSHKeyPath            string    `json:"ssh_key_path,omitempty"`
	SSHPort               int       `json:"ssh_port,omitempty"` // default 22
	WorkingDir            string    `json:"working_dir,omitempty"`
	MaxConcurrentSessions int       `json:"max_concurrent_sessions"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// InstanceCredential binds a credential to a harness instance with priority and concurrency limits.
type InstanceCredential struct {
	InstanceID    string `json:"instance_id"`
	CredentialID  string `json:"credential_id"`
	Priority      int    `json:"priority"`       // 0 = primary, 1+ = fallbacks
	MaxConcurrent int    `json:"max_concurrent"` // how many sessions can use this cred simultaneously
	Enabled       bool   `json:"enabled"`
}

// CredentialSlot tracks active usage of a credential by sessions.
type CredentialSlot struct {
	InstanceID   string `json:"instance_id"`
	CredentialID string `json:"credential_id"`
	SessionID    string `json:"session_id"`
	AcquiredAt   time.Time `json:"acquired_at"`
}

// InstanceStatus aggregates runtime status for an instance.
type InstanceStatus struct {
	Instance       Instance             `json:"instance"`
	ActiveSessions int                  `json:"active_sessions"`
	Credentials    []CredentialStatus   `json:"credentials"`
	Reachable      bool                 `json:"reachable"` // SSH connectivity check
	LastChecked    time.Time            `json:"last_checked"`
}

// CredentialStatus shows availability of a credential on an instance.
type CredentialStatus struct {
	CredentialID  string `json:"credential_id"`
	Priority      int    `json:"priority"`
	MaxConcurrent int    `json:"max_concurrent"`
	InUse         int    `json:"in_use"`
	Available     int    `json:"available"` // max_concurrent - in_use
	Enabled       bool   `json:"enabled"`
}
