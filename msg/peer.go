package msg

import "time"

// Peers: other llm-bridge-servers linked to one another, each direction on
// its own. These are the records and bodies of llm-bridge-server's peer
// routes; the server's store embeds them beside the hashes it never sends.
// Design note d107154a-49ac-471b-a1b7-fc7975de37f3.

// PeerInvite is a one-time passphrase an administrator on the issuing server
// hands to another server, so that it can act there as PeerPrincipalID.
type PeerInvite struct {
	ID              string     `json:"id"`
	PeerPrincipalID string     `json:"peer_principal_id"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RedeemedAt      *time.Time `json:"redeemed_at,omitempty"`
	// PeerTokenID is the token the redemption made, empty until then.
	PeerTokenID string `json:"peer_token_id,omitempty"`
}

// PeerToken is the credential another server presents to act on the issuing
// server as PeerPrincipalID. The token itself is handed once, to the server
// that redeemed the invite.
type PeerToken struct {
	ID              string     `json:"id"`
	PeerPrincipalID string     `json:"peer_principal_id"`
	PeerInviteID    string     `json:"peer_invite_id"`
	CreatedAt       time.Time  `json:"created_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	RevokedBy       string     `json:"revoked_by,omitempty"`
	RevokeReason    string     `json:"revoke_reason,omitempty"`
}

// PeerLink is another server the calling server holds a peer token for.
type PeerLink struct {
	ID string `json:"id"`
	// BaseURL is the other server's https address, as the operator gave it.
	BaseURL     string `json:"base_url"`
	DisplayName string `json:"display_name"`
	// RemotePeerPrincipalID is the principal the calling server acts as on
	// the other one, in the other server's principal-store.
	RemotePeerPrincipalID string `json:"remote_peer_principal_id"`
	// RemotePeerTokenID is the other server's id for the token, which its
	// operator revokes it by.
	RemotePeerTokenID string `json:"remote_peer_token_id"`
	// CredentialID is the auth-store credential that holds the token.
	CredentialID string     `json:"credential_id"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	RemovedAt    *time.Time `json:"removed_at,omitempty"`
	RemovedBy    string     `json:"removed_by,omitempty"`
	RemoveReason string     `json:"remove_reason,omitempty"`
}

// PeerInviteRequest is the body of POST /peer-invites. ExpiresInSeconds 0
// takes the server's default.
type PeerInviteRequest struct {
	PeerPrincipalID  string `json:"peer_principal_id"`
	ExpiresInSeconds int64  `json:"expires_in_seconds,omitempty"`
}

// PeerInviteCreated answers POST /peer-invites: the invite and its
// passphrase, which is shown here once and kept nowhere.
type PeerInviteCreated struct {
	Invite     PeerInvite `json:"invite"`
	Passphrase string     `json:"passphrase"`
}

// PeerRedeemRequest is the body of POST /peer/redeem.
type PeerRedeemRequest struct {
	Passphrase string `json:"passphrase"`
}

// PeerRedeemed answers POST /peer/redeem: the token, shown here once, and
// what it acts as.
type PeerRedeemed struct {
	Token           string `json:"token"`
	PeerTokenID     string `json:"peer_token_id"`
	PeerPrincipalID string `json:"peer_principal_id"`
}

// PeerSelf answers GET /peer/self: who a peer token acts as on the server
// that issued it. POST /peer-links/{id}/check passes it back unchanged.
type PeerSelf struct {
	PeerTokenID     string `json:"peer_token_id"`
	PeerPrincipalID string `json:"peer_principal_id"`
	IsAdministrator bool   `json:"is_administrator"`
}

// PeerLinkRequest is the body of POST /peer-links.
type PeerLinkRequest struct {
	BaseURL     string `json:"base_url"`
	Passphrase  string `json:"passphrase"`
	DisplayName string `json:"display_name"`
}

// PeerReasonRequest is the body of POST /peer-tokens/{id}/revoke and
// POST /peer-links/{id}/remove. The reason stays on the row.
type PeerReasonRequest struct {
	Reason string `json:"reason"`
}

// PeerLinkRemoved answers POST /peer-links/{id}/remove. Note says the token
// still works on the other server until its operator revokes it there.
type PeerLinkRemoved struct {
	Link PeerLink `json:"link"`
	Note string   `json:"note"`
}
