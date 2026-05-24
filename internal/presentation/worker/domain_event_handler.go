package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainSvc "github.com/yourname/go-clean-base/internal/domain/service"
)

// DomainEventHandler processes Kafka domain events and publishes downstream tasks.
// It is the only component allowed to call INotificationPublisher — the usecase
// must never call RabbitMQ directly.
type DomainEventHandler struct {
	notifPublisher domainSvc.INotificationPublisher
}

func NewDomainEventHandler(p domainSvc.INotificationPublisher) *DomainEventHandler {
	return &DomainEventHandler{notifPublisher: p}
}

// Handle unmarshals the Kafka message, logs the event, and publishes a
// notification task to RabbitMQ for create events.
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

	if event.EventType == domainModel.EventTypeTodoCreated {
		n := &domainModel.Notification{
			To:      "admin@example.com",
			Subject: "New Todo Created",
			Body:    "Todo aggregate_id: " + event.AggregateID,
		}
		if err := h.notifPublisher.PublishNotification(context.Background(), n); err != nil {
			slog.Error("notification publish failed", "error", err, "event_id", event.EventID)
		}
	}

	return nil
}
