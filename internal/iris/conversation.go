package iris

import "time"

const (
	defaultInvitationLifetime = 2 * time.Hour
	defaultInactivityLifetime = 24 * time.Hour
	maxActiveConversations    = 64
	maxMessageBodyBytes       = 256 << 10
	maxConversationMessages   = 10000
	maxConversationBodyBytes  = 64 << 20
)

type participant struct {
	ProjectID string     `json:"project_id"`
	Alias     string     `json:"alias"`
	JoinedAt  *time.Time `json:"joined_at"`
}

type message struct {
	SchemaVersion   int       `json:"schema_version"`
	ID              string    `json:"message_id"`
	ConversationID  string    `json:"conversation_id"`
	Sequence        int64     `json:"sequence"`
	SenderProjectID *string   `json:"sender_project_id"`
	SenderAlias     string    `json:"sender_alias"`
	Recipients      []string  `json:"recipients"`
	CreatedAt       time.Time `json:"created_at"`
	Kind            string    `json:"kind"`
	Body            string    `json:"body"`
	Topic           string    `json:"topic"`
	ReplyTo         *string   `json:"reply_to"`
	CorrelationID   *string   `json:"correlation_id"`
	IdempotencyKey  *string   `json:"idempotency_key"`
}

type conversation struct {
	SchemaVersion       int           `json:"schema_version"`
	ID                  string        `json:"conversation_id"`
	Ordinal             int64         `json:"ordinal"`
	Label               string        `json:"label"`
	Topic               string        `json:"topic"`
	State               string        `json:"state"`
	CreatedAt           time.Time     `json:"created_at"`
	LastActivityAt      time.Time     `json:"last_activity_at"`
	TerminalAt          *time.Time    `json:"terminal_at"`
	Participants        []participant `json:"participants"`
	InvitationCode      string        `json:"invitation_code"`
	InvitationExpiresAt time.Time     `json:"invitation_expires_at"`
	InitialMessage      message       `json:"initial_message"`
}

func milliseconds(value int64) time.Time {
	return time.UnixMilli(value).UTC()
}
