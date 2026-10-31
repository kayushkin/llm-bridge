package msg

// CallerSelf is llm-bridge-server's answer to GET /self: who the request
// acts as. A service calling with its service principal token reads its own
// principal id here, so the id is never configured beside the token.
type CallerSelf struct {
	// PrincipalID is the principal-store id the request acts as, empty for
	// the internal service acting as itself.
	PrincipalID string `json:"principal_id,omitempty"`
	// IsAdministrator is principal-store's answer for that principal.
	IsAdministrator bool `json:"is_administrator"`
	// InternalService is set for the service token with no principal named.
	InternalService bool `json:"internal_service"`
}
