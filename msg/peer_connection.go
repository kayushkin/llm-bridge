package msg

import "time"

// Peer connections: how any two llm-bridge-servers reach each other, and what
// each lets the other do. Design note d107154a-49ac-471b-a1b7-fc7975de37f3,
// "Decided on 2026-10-27".
//
// A connection says how two servers reach each other and who they are; it
// gives no access. A grant of access is a peer principal and token that the
// granting server makes for the other server and delivers over the
// connection. Each server grants access on itself, and revokes it, on its own.

// PeerTransport is how two connected servers carry requests to each other.
type PeerTransport string

const (
	// PeerTransportDirect: both servers have an https address, and each calls
	// the other at it.
	PeerTransportDirect PeerTransport = "direct"
	// PeerTransportDialOut: only one server has an address. The other dials
	// out to it and keeps a WebSocket open, and requests in both directions
	// travel over that socket.
	PeerTransportDialOut PeerTransport = "dial_out"
)

// PeerTransports is every transport, for a UI that must not restate them.
var PeerTransports = []PeerTransport{PeerTransportDirect, PeerTransportDialOut}

// PeerConnectionRole says which side of the setup this server was.
type PeerConnectionRole string

const (
	// PeerConnectionRoleAccepting: this server minted the connection
	// passphrase. It has an address; it never dials.
	PeerConnectionRoleAccepting PeerConnectionRole = "accepting"
	// PeerConnectionRoleInitiating: this server's administrator entered the
	// other server's address and passphrase. On a dial_out connection it is
	// the one that dials.
	PeerConnectionRoleInitiating PeerConnectionRole = "initiating"
)

// PeerConnectionInvite is a one-time passphrase an administrator on a server
// with an address hands to another server's administrator, so that server can
// connect to this one. Connecting gives no access.
type PeerConnectionInvite struct {
	ID         string     `json:"id"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
	// ConnectionID is the connection the redemption made, empty until then.
	ConnectionID string `json:"connection_id,omitempty"`
}

// PeerConnectionInviteRequest is the body of POST /peer-connection-invites.
// ExpiresInSeconds 0 takes the server's default.
type PeerConnectionInviteRequest struct {
	ExpiresInSeconds int64 `json:"expires_in_seconds,omitempty"`
}

// PeerConnectionInviteCreated answers POST /peer-connection-invites: the
// invite and its passphrase, shown here once and kept nowhere.
type PeerConnectionInviteCreated struct {
	Invite     PeerConnectionInvite `json:"invite"`
	Passphrase string               `json:"passphrase"`
}

// PeerConnection is this server's record of another server it is connected
// to. Each side keeps its own row, with its own id; RemoteConnectionID is the
// other side's.
type PeerConnection struct {
	ID string `json:"id"`
	// DisplayName is what this server's administrator calls the other server.
	DisplayName string             `json:"display_name"`
	Transport   PeerTransport      `json:"transport"`
	Role        PeerConnectionRole `json:"role"`
	// RemoteBaseURL is the other server's https address, empty when it has
	// none (the dialing side of a dial_out connection, seen from the side it
	// dials).
	RemoteBaseURL string `json:"remote_base_url,omitempty"`
	// RemoteConnectionID is the other side's id for the connection. Only the
	// initiating side knows it, from the accepting side's answer; the
	// accepting side is never told the initiating side's.
	RemoteConnectionID string `json:"remote_connection_id,omitempty"`
	// PeerPrincipalID is the principal this server made for the other server
	// when it first granted it access, empty until then. It is this server's
	// principal-store's id.
	PeerPrincipalID string     `json:"peer_principal_id,omitempty"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	RemovedAt       *time.Time `json:"removed_at,omitempty"`
	// RemovedBy is the administrator here who removed the connection, empty
	// when the other server removed it and said so (RemovedByOtherServer).
	RemovedBy    string `json:"removed_by,omitempty"`
	RemoveReason string `json:"remove_reason,omitempty"`
	// RemovedByOtherServer is set when the other server removed the
	// connection and told this one over it.
	RemovedByOtherServer bool `json:"removed_by_other_server,omitempty"`
	// RemovalNotice says whether this server told the other one it removed
	// the connection, or why it could not. The connection is removed here
	// either way.
	RemovalNotice string `json:"removal_notice,omitempty"`
}

// PeerConnectionView is a connection with what is true of it now: whether its
// socket is up, the access this server gives the other, and the access this
// server holds on the other.
type PeerConnectionView struct {
	Connection PeerConnection `json:"connection"`
	// SocketConnected is set on a dial_out connection when its WebSocket is
	// open now. It is always false on a direct one, which has no socket.
	SocketConnected bool `json:"socket_connected"`
	// AccessGiven is this server's live grant to the other server, if any.
	AccessGiven *PeerGrant `json:"access_given,omitempty"`
	// AccessHeld is the live access the other server gave this one, if any.
	AccessHeld *PeerAccess `json:"access_held,omitempty"`
}

// PeerConnectRequest is the body of POST /peer-connections, on the server
// whose administrator enters the other server's address and passphrase.
type PeerConnectRequest struct {
	// BaseURL is the other server's https address.
	BaseURL    string `json:"base_url"`
	Passphrase string `json:"passphrase"`
	// DisplayName is what this server will call the other one.
	DisplayName string `json:"display_name"`
	// NameForThisServer is what the other server will call this one.
	NameForThisServer string `json:"name_for_this_server"`
	// ThisServerBaseURL is this server's own https address, when it has one.
	// With it both servers call each other directly; without it this server
	// dials out and keeps a socket open.
	ThisServerBaseURL string `json:"this_server_base_url,omitempty"`
}

// PeerConnectCall is the body one server posts to the other's POST
// /peer/connect. It is server-to-server and never comes from a person.
type PeerConnectCall struct {
	Passphrase string `json:"passphrase"`
	// ServerName is what the accepting server will call the initiating one.
	ServerName string `json:"server_name"`
	// BaseURL is the initiating server's own https address, empty when it has
	// none and will dial out.
	BaseURL string `json:"base_url,omitempty"`
	// Credential is what the accepting server presents to the initiating one
	// on connection calls. The initiating server keeps only its hash.
	Credential string `json:"credential"`
}

// PeerConnectAnswer answers POST /peer/connect.
type PeerConnectAnswer struct {
	// ConnectionID is the accepting server's id for the connection.
	ConnectionID string        `json:"connection_id"`
	Transport    PeerTransport `json:"transport"`
	// Credential is what the initiating server presents to the accepting one
	// on connection calls and to open the socket. The accepting server keeps
	// only its hash. It gives no access of its own.
	Credential string `json:"credential"`
}

// PeerAccessLevelNone and PeerAccessLevelAdministrator are the two levels
// that need no group: a peer principal with nothing, and one principal-store
// marks an administrator. PeerAccessLevelGroup names any principal-store
// group. Every other level is served by GET /peer-access-levels.
const (
	PeerAccessLevelNone          = "none"
	PeerAccessLevelAdministrator = "administrator"
	PeerAccessLevelGroup         = "group"
)

// PeerAccessLevel is one level of access a server can grant another, as GET
// /peer-access-levels serves it.
type PeerAccessLevel struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	// Grants are the grant-store grants the level's group holds, for a level
	// that has a group.
	Grants []PeerAccessLevelGrant `json:"grants,omitempty"`
	// GroupPrincipalID is the principal-store group a peer at this level is
	// put in, empty for none and administrator and for a level not yet
	// provisioned on this server.
	GroupPrincipalID string `json:"group_principal_id,omitempty"`
	// NeedsGroup is true for a level that works only once provisioned.
	NeedsGroup bool `json:"needs_group"`
}

// PeerAccessLevelGrant is one grant a level's group holds.
type PeerAccessLevelGrant struct {
	Relation     string `json:"relation"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

// PeerGrantRequest is the body of POST /peer-connections/{id}/grant. Level is
// a key from GET /peer-access-levels, or "group" with GroupPrincipalID naming
// any principal-store group.
type PeerGrantRequest struct {
	Level            string `json:"level"`
	GroupPrincipalID string `json:"group_principal_id,omitempty"`
}

// PeerGrant is access this server gave another: the peer principal it acts
// as here, the level, and the token, which only the other server holds.
type PeerGrant struct {
	ID              string `json:"id"`
	ConnectionID    string `json:"connection_id"`
	PeerPrincipalID string `json:"peer_principal_id"`
	Level           string `json:"level"`
	// GroupPrincipalID is the group the level put the peer in, if any.
	GroupPrincipalID string    `json:"group_principal_id,omitempty"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	// DeliveredAt is when the other server confirmed it holds the token.
	// Empty means delivery failed and the grant cannot be used yet.
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    string     `json:"revoked_by,omitempty"`
	RevokeReason string     `json:"revoke_reason,omitempty"`
}

// PeerAccess is access another server gave this one: the token it holds in
// auth-store and what it acts as there.
type PeerAccess struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connection_id"`
	// RemoteGrantID and RemotePeerPrincipalID are the other server's ids.
	RemoteGrantID         string `json:"remote_grant_id"`
	RemotePeerPrincipalID string `json:"remote_peer_principal_id"`
	Level                 string `json:"level"`
	// CredentialID is the auth-store credential that holds the token.
	CredentialID string     `json:"credential_id"`
	ReceivedAt   time.Time  `json:"received_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	EndReason    string     `json:"end_reason,omitempty"`
}

// PeerAccessDelivery is the body of POST /peer/access, server to server: the
// granting server hands over the token, once.
type PeerAccessDelivery struct {
	GrantID         string `json:"grant_id"`
	PeerPrincipalID string `json:"peer_principal_id"`
	Level           string `json:"level"`
	Token           string `json:"token"`
}

// PeerAccessRevocation is the body of POST /peer/access/revoked, server to
// server: the granting server says a grant no longer works.
type PeerAccessRevocation struct {
	GrantID string `json:"grant_id"`
	Reason  string `json:"reason"`
}

// PeerGrantRevoked answers POST /peer-connections/{id}/revoke-grant. Notice
// says whether the other server was told; the grant is revoked here either
// way.
type PeerGrantRevoked struct {
	Grant  PeerGrant `json:"grant"`
	Notice string    `json:"notice"`
}

// PeerGrantSelf answers GET /peer/self: who the presented peer token acts as
// on the server that issued it. POST /peer-connections/{id}/check passes it
// back unchanged.
type PeerGrantSelf struct {
	GrantID         string `json:"grant_id"`
	ConnectionID    string `json:"connection_id"`
	PeerPrincipalID string `json:"peer_principal_id"`
	Level           string `json:"level"`
	IsAdministrator bool   `json:"is_administrator"`
}

// PeerConnectionRemovedNotice is the body of POST /peer/connection/removed,
// server to server: the other server removed the connection, and this one
// marks it removed too rather than finding out from refused calls.
type PeerConnectionRemovedNotice struct {
	Reason string `json:"reason"`
}

// PeerUserMapping says which of this server's principals a user of a
// connected server acts as when that server names the user on a request
// (phase 2 of design note d107154a). The request is then allowed only what
// both the peer principal's grants and LocalPrincipalID's grants allow. It is
// keyed on ids: the peer principal this server made for the other server, and
// the other server's own id for its user.
type PeerUserMapping struct {
	ID string `json:"id"`
	// PeerPrincipalID is this server's principal for the other server.
	PeerPrincipalID string `json:"peer_principal_id"`
	// RemotePrincipalID is the other server's principal-store id for its
	// user. It is only ever compared, never looked up here.
	RemotePrincipalID string `json:"remote_principal_id"`
	// LocalPrincipalID is this server's principal the user acts as.
	LocalPrincipalID string     `json:"local_principal_id"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	RemovedAt        *time.Time `json:"removed_at,omitempty"`
	RemovedBy        string     `json:"removed_by,omitempty"`
	RemoveReason     string     `json:"remove_reason,omitempty"`
}

// PeerUserMappingRequest is the body of POST /peer-user-mappings.
type PeerUserMappingRequest struct {
	PeerPrincipalID   string `json:"peer_principal_id"`
	RemotePrincipalID string `json:"remote_principal_id"`
	LocalPrincipalID  string `json:"local_principal_id"`
}
