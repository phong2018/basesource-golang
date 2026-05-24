package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainService "github.com/yourname/go-clean-base/internal/domain/service"
)

// kafkaProducer implements IEventPublisher using segmentio/kafka-go.
// It is the Kafka side of the dual-broker setup: every domain event that
// passes through the OutboxRelay is written here as a durable Kafka message.
type kafkaProducer struct {
	// writer is a long-lived kafka.Writer configured in kafka_client.go.
	// It holds a connection pool to the broker and handles retries internally.
	writer *kafka.Writer
}

// NewKafkaProducer wraps a pre-configured kafka.Writer and returns it as
// the IEventPublisher interface. Called once at startup from the container.
func NewKafkaProducer(writer *kafka.Writer) domainService.IEventPublisher {
	return &kafkaProducer{writer: writer}
}

// Publish serialises the OutboxEvent to JSON and writes it to the Kafka topic.
//
// Partition key = AggregateID (e.g. todo ID).
// The kafka.Hash{} balancer hashes the key to a partition number, so all
// events that share the same AggregateID always land in the same partition.
// This guarantees strict ordering: todo.created → todo.updated → todo.deleted
// for a given entity are always consumed in the correct sequence.
//
// Headers carry event_type and event_id so consumers can route or filter
// without deserialising the body.
//
// WriteMessages blocks until the broker acknowledges the write
// (RequiredAcks: RequireAll is set on the writer).
func (p *kafkaProducer) Publish(ctx context.Context, event *domainModel.OutboxEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID), // partition key = todo ID → ordering per todo
		Value: body,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "event_id", Value: []byte(event.EventID)},
		},
	})
}

// Close flushes any buffered messages and closes the connection to the broker.
// Called from the worker defer block on graceful shutdown.
func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
