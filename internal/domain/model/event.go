package model

import "time"

// Entity: OutboxEvent maps 1:1 to the outbox_events table row.
type OutboxEvent struct {
	ID            uint      `db:"id"`
	EventID       string    `db:"event_id"`
	EventType     string    `db:"event_type"`
	AggregateType string    `db:"aggregate_type"`
	AggregateID   string    `db:"aggregate_id"`
	Payload       string    `db:"payload"`
	CreatedAt     time.Time `db:"created_at"`
}

// Entity: OutboxDelivery maps 1:1 to the outbox_deliveries table row.
// Each OutboxEvent has one delivery row per destination broker.
type OutboxDelivery struct {
	ID            uint       `db:"id"`
	OutboxEventID uint       `db:"outbox_event_id"`
	Destination   string     `db:"destination"`
	Status        string     `db:"status"`
	AttemptCount  int        `db:"attempt_count"`
	LastError     *string    `db:"last_error"`
	PublishedAt   *time.Time `db:"published_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

// Domain Rule Constants: outbox event types
const (
	EventTypeTodoCreated = "todo.created"
	EventTypeTodoUpdated = "todo.updated"
	EventTypeTodoDeleted = "todo.deleted"
)
