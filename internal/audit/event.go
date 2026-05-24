package audit

// AuditEvent represents an auditable action (shorten or follow).
// generate:reset
type AuditEvent struct {
	Timestamp int64  `json:"ts"`
	Action    Action `json:"action"`
	UserID    string `json:"user_id,omitempty"`
	URL       string `json:"url"`
}

// Action is the type of audit event.
type Action string

// Audit action constants.
const (
	// ActionShorten is logged when a URL is shortened.
	ActionShorten Action = "shorten"
	// ActionFollow is logged when a short URL is followed (redirected).
	ActionFollow Action = "follow"
)
