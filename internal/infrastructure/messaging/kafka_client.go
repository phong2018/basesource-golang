package messaging

import (
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/yourname/go-clean-base/config"
)

// NewKafkaWriter creates a synchronous Kafka producer (kafka.Writer).
//
// Configuration choices:
//   - Balancer: kafka.Hash{} — hashes the message Key to select a partition.
//     Using AggregateID as the key means all events for the same entity always
//     land in the same partition, guaranteeing strict ordering per entity.
//   - RequiredAcks: RequireAll — the broker leader waits for all in-sync replicas
//     to acknowledge the write before returning. Strongest durability guarantee;
//     prevents data loss if the leader fails immediately after a write.
//   - Async: false — WriteMessages blocks until the broker confirms the write.
//     This is intentional: the OutboxRelay must know whether publish succeeded
//     before marking the outbox row as published.
func NewKafkaWriter(cfg *config.MessagingConfig) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        cfg.KafkaTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
}

// NewKafkaReader creates a Kafka consumer bound to a named consumer group.
//
// Configuration choices:
//   - GroupID: consumers in the same group share partition assignments.
//     Kafka ensures each partition is read by exactly one member of the group,
//     enabling horizontal scaling — add more worker instances to process in parallel.
//   - StartOffset: FirstOffset — when the consumer group has no committed offset
//     (first run, or group was reset), start reading from the very beginning of
//     the partition instead of the default LastOffset (which would skip all
//     existing messages). This enables event replay / reprocessing.
//   - MinBytes / MaxBytes: fetch batch size hints. MinBytes=1 means the broker
//     returns as soon as at least 1 byte is available (low latency).
//     MaxBytes=10MB caps the maximum fetch size per request.
func NewKafkaReader(cfg *config.MessagingConfig) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:           cfg.KafkaBrokers,
		Topic:             cfg.KafkaTopic,
		GroupID:           cfg.KafkaGroupID,
		StartOffset:       kafka.FirstOffset,
		MinBytes:          1,
		MaxBytes:          10e6,
		SessionTimeout:    6 * time.Second,
		HeartbeatInterval: 2 * time.Second,
		MaxWait:           500 * time.Millisecond,
	})
}
