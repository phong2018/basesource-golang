package model

// Value Object: Notification is a data carrier for outgoing messages.
// No identity — the notification service decides delivery details.
type Notification struct {
	To      string
	Subject string
	Body    string
}
