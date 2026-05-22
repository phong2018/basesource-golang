package dto

type NotificationRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type NotificationResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}
