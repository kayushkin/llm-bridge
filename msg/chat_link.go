package msg

import "time"

// A chat link ties one session to one conversation outside the bridge — a
// person writing to one of our bot accounts on Discord — so that the person
// and the session can talk. llm-bridge-server reads what the person sends
// from the message bus, puts it into the session as a user message labeled
// as theirs, and sends the final reply of each turn it started back to them
// through multichat, as the same account. Turns anyone else starts in the
// session are never sent.

// ChatLinkCreate is the body of POST /sessions/{id}/chat-links.
type ChatLinkCreate struct {
	// AccountID is multichat's name for the account the conversation is on:
	// message-store's account id of one of our bot accounts
	// (message_account_000001). GET multichat /api/accounts lists them.
	AccountID string `json:"account_id"`
	// Platform is the account's platform, as multichat names it (discord).
	Platform string `json:"platform"`
	// ConversationID is the platform's id of the conversation: on Discord the
	// direct-message channel id between the bot and the person.
	ConversationID string `json:"conversation_id"`
	// PersonDisplayName is what the session calls the person, for display
	// and for the label on what they send. Never used to find them.
	PersonDisplayName string `json:"person_display_name"`
}

// ChatLink is one session's link to one conversation.
type ChatLink struct {
	ID                string `json:"id"` // chat_link_000001
	SessionID         string `json:"session_id"`
	AccountID         string `json:"account_id"`
	Platform          string `json:"platform"`
	ConversationID    string `json:"conversation_id"`
	PersonDisplayName string `json:"person_display_name"`
	// CreatedByPrincipalID is the principal the creating request acted as;
	// CreatedBySessionAgent is set when the session's own agent made it.
	CreatedByPrincipalID  string    `json:"created_by_principal_id,omitempty"`
	CreatedBySessionAgent bool      `json:"created_by_session_agent,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	// EndedAt is zero while the link is active. Messages that arrive after
	// it ends are not kept.
	EndedAt time.Time `json:"ended_at,omitzero"`
	// Messages is the link's log, oldest first, when one link is read.
	Messages []ChatLinkMessage `json:"messages,omitempty"`
}

// ChatLinkDirection says which way a link message went.
type ChatLinkDirection string

const (
	// ChatLinkInbound is a message the person sent, on its way into the
	// session.
	ChatLinkInbound ChatLinkDirection = "inbound"
	// ChatLinkOutbound is a turn's final reply, on its way to the person.
	ChatLinkOutbound ChatLinkDirection = "outbound"
)

// ChatLinkMessageState is where a link message is.
type ChatLinkMessageState string

const (
	// ChatLinkMessagePending is waiting to be delivered: an inbound message
	// waits for the session to be free, an outbound one for multichat.
	ChatLinkMessagePending ChatLinkMessageState = "pending"
	// ChatLinkMessageInSession is an inbound message the session has been
	// sent, whose turn has not ended yet.
	ChatLinkMessageInSession ChatLinkMessageState = "in_session"
	// ChatLinkMessageDelivered is an inbound message whose turn ended, or an
	// outbound one the platform took.
	ChatLinkMessageDelivered ChatLinkMessageState = "delivered"
	// ChatLinkMessageFailed is a message that will not be delivered; Error
	// says why. An outbound one is retried until it is delivered or the
	// link's retries run out.
	ChatLinkMessageFailed ChatLinkMessageState = "failed"
)

// ChatLinkMessage is one message of a link's log.
type ChatLinkMessage struct {
	ID        string               `json:"id"` // chat_link_message_000001
	LinkID    string               `json:"link_id"`
	Direction ChatLinkDirection    `json:"direction"`
	State     ChatLinkMessageState `json:"state"`
	Body      string               `json:"body"`
	// Attachments are the files an inbound message came with.
	Attachments []ChatLinkAttachment `json:"attachments,omitempty"`
	// PlatformMessageIDs are the platform's ids: the message the person sent,
	// or each part the reply was sent as.
	PlatformMessageIDs []string `json:"platform_message_ids,omitempty"`
	// BusEventID is the inbound message's messaging.inbound event id, its
	// idempotency key.
	BusEventID string `json:"bus_event_id,omitempty"`
	// DeliveryID ties an inbound message to the session turn it went into
	// and the reply that turn produced: the client_request_id of that turn,
	// shared by every inbound message sent together and by the outbound
	// reply.
	DeliveryID string `json:"delivery_id,omitempty"`
	// BridgeMessageID is the user message the session was sent (inbound).
	BridgeMessageID string    `json:"bridge_message_id,omitempty"`
	Attempts        int       `json:"attempts"`
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// NextAttemptAt is when a pending outbound message is tried again.
	NextAttemptAt time.Time `json:"next_attempt_at,omitzero"`
}

// ChatLinkAttachment is one file a person sent.
type ChatLinkAttachment struct {
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int    `json:"size_bytes"`
}
