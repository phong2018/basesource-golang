package model

import "time"

// Entity: OutboxEvent represents a pending domain event to be published to the message broker.
type OutboxEvent struct {
	ID          uint       `db:"id"`
	EventID     string     `db:"event_id"`
	EventType   string     `db:"event_type"`
	AggregateID string     `db:"aggregate_id"`
	Payload     string     `db:"payload"`
	Status      string     `db:"status"`
	CreatedAt   time.Time  `db:"created_at"`
	PublishedAt *time.Time `db:"published_at"`
}

// Domain Rule Constants: outbox event types
const (
	EventTypeTodoCreated = "todo.created"
	EventTypeTodoUpdated = "todo.updated"
	EventTypeTodoDeleted = "todo.deleted"
)

// Domain Rule Constants: outbox event statuses
const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)
