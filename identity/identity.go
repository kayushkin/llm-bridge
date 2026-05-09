// Package identity tracks the canonical bridge message_id for events flowing
// through a harness adapter. Adapters use it to:
//   - Mint message_id on bubble boundaries (split detection when the harness
//     starts a new message inside the same turn).
//   - Reconcile against prior message_ids on resume, when the harness
//     re-emits events it produced before the bridge crashed or restarted.
//
// Pre-Phase III.B this logic lived in llm-bridge-server's harness manager
// (assignAssistantID + HarnessToBridgeMap). After Phase III.B it moves out to
// each harness adapter so bridge-server stops needing harness_message_id.
// See MIGRATION-session-identity.md.
package identity

// Store is the persistence interface adapters implement. It is per-session;
// each session's Tracker owns its own Store. Errors are propagated to the
// adapter caller — the adapter decides whether to retry or surface them.
type Store interface {
	// Lookup returns the bridge message_id previously bound to harnessID,
	// or ("", false, nil) if none. err non-nil only on backend failure.
	Lookup(harnessID string) (messageID string, found bool, err error)

	// Put binds harnessID to messageID. Idempotent; repeat calls with the
	// same pair are no-ops.
	Put(harnessID, messageID string) error
}

// MintFn is the callback Tracker invokes to allocate a fresh bridge
// message_id when split detection triggers. Adapters pass their own
// implementation (typically wrapping a ULID generator) so the package
// stays dependency-free.
type MintFn func() string

// Tracker holds the open-bubble state for one session and consults Store
// for resume reconciliation. Not safe for concurrent use — adapters should
// serialize per-session through their event dispatcher.
type Tracker struct {
	store Store
	mint  MintFn

	// openMessageID is the currently-open assistant bridge message_id.
	// Empty between bubbles (after EndTurn or before the first bubble).
	openMessageID string

	// lastHarnessID is the harness-native id of the last event Tracker saw.
	// Empty when the open bubble has no harness id yet, or between bubbles.
	lastHarnessID string
}

// NewTracker constructs a Tracker bound to a per-session store and minter.
// Both arguments are required — passing nil is a programmer error and will
// panic on first use.
func NewTracker(store Store, mint MintFn) *Tracker {
	return &Tracker{store: store, mint: mint}
}

// AssignMessageID picks a bridge message_id for an event whose harness-native
// id is hid. Empty hid means the event has no native correlation (e.g. a
// stream delta without a parent harness message id) and the open bubble is
// reused.
//
// Behavior:
//   - If hid is non-empty and Store has a prior binding for it, reuse the
//     bound message_id (resume reconciliation; keeps re-emitted events in
//     their original bubble).
//   - If hid is non-empty and differs from lastHarnessID with a bubble
//     already open, the harness has moved to a new bubble inside the same
//     turn — mint a fresh message_id.
//   - Otherwise, reuse the open message_id (or mint one if no bubble is
//     open yet).
//
// Side effect: when hid is non-empty and not previously bound, Store.Put
// records the new binding for future resume reconciliation.
func (t *Tracker) AssignMessageID(hid string) (string, error) {
	if hid != "" {
		existing, found, err := t.store.Lookup(hid)
		if err != nil {
			return "", err
		}
		if found {
			t.openMessageID = existing
			t.lastHarnessID = hid
			return existing, nil
		}
	}

	// Split detection: harness moved to a new bubble inside the same turn.
	if hid != "" && t.lastHarnessID != "" && t.lastHarnessID != hid {
		t.openMessageID = ""
	}

	if t.openMessageID == "" {
		t.openMessageID = t.mint()
	}

	if hid != "" {
		if err := t.store.Put(hid, t.openMessageID); err != nil {
			return "", err
		}
		t.lastHarnessID = hid
	}
	return t.openMessageID, nil
}

// EndTurn clears the open bubble at turn terminator events (result, error).
// The next AssignMessageID call after EndTurn always opens a fresh bubble
// (or reuses a prior binding from Store on resume).
func (t *Tracker) EndTurn() {
	t.openMessageID = ""
	t.lastHarnessID = ""
}

// OpenMessageID returns the currently-open bridge message_id, or "" if no
// bubble is open. Useful for adapters that need to peek without assigning,
// e.g. when stamping a tool_use_id binding for later correlation.
func (t *Tracker) OpenMessageID() string {
	return t.openMessageID
}
