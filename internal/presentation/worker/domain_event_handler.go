package worker

import (
	"encoding/json"
	"log/slog"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
)

// DomainEventHandler processes Kafka domain events for audit log / analytics.
type DomainEventHandler struct{}

func NewDomainEventHandler() *DomainEventHandler {
	return &DomainEventHandler{}
}

// Handle unmarshals the Kafka message and logs the domain event.
func (h *DomainEventHandler) Handle(body []byte) error {
	var event domainModel.OutboxEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}
	slog.Info("domain event received",
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
		"event_id", event.EventID,
	)
	return nil
}
