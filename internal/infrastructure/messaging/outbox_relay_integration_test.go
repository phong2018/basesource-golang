package messaging_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	repoMock "github.com/yourname/go-clean-base/internal/domain/repository/mock"
	"github.com/yourname/go-clean-base/internal/infrastructure/messaging"
)

const testBroker = "localhost:9092"

// kafkaAvailable returns true if the broker TCP port is reachable.
func kafkaAvailable() bool {
	d := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(context.Background(), "tcp", testBroker)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// uniqueTopic generates a test-scoped topic so parallel runs don't collide.
func uniqueTopic(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-outbox-%d", time.Now().UnixNano())
}

// createTopic pre-creates a topic via the Kafka admin API so the producer
// doesn't hit "Unknown Topic" on its first write while auto-create is pending.
func createTopic(t *testing.T, topic string) {
	t.Helper()

	conn, err := kafka.Dial("tcp", testBroker)
	if err != nil {
		t.Fatalf("kafka.Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		t.Fatalf("Controller: %v", err)
	}

	ctrl, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		t.Fatalf("dial controller: %v", err)
	}
	defer func() { _ = ctrl.Close() }()

	if err := ctrl.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}); err != nil {
		t.Fatalf("CreateTopics: %v", err)
	}
}

// TestOutboxRelay_PublishesToKafka is a real integration test.
// It requires a running Kafka broker at localhost:9092.
// Skip with -short if the broker is not available.
func TestOutboxRelay_PublishesToKafka(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Kafka integration test in short mode")
	}
	if !kafkaAvailable() {
		t.Skip("Kafka not reachable at " + testBroker + " — start docker-compose first")
	}

	topic := uniqueTopic(t)
	createTopic(t, topic)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ── producer (kafka writer) ───────────────────────────────────────────────
	writer := &kafka.Writer{
		Addr:         kafka.TCP(testBroker),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
	defer func() { _ = writer.Close() }()

	producer := messaging.NewKafkaProducer(writer)
	defer func() { _ = producer.Close() }()

	// ── stub event ────────────────────────────────────────────────────────────
	event := &domainModel.OutboxEvent{
		ID:          1,
		EventID:     "evt-abc123",
		EventType:   domainModel.EventTypeTodoCreated,
		AggregateID: "42",
		Payload:     `{"title":"Buy milk"}`,
		Status:      domainModel.OutboxStatusPending,
		CreatedAt:   time.Now(),
	}

	// ── mock repo: one pending event, records MarkPublished calls ─────────────
	marked := false
	repo := &repoMock.OutboxRepositoryMock{
		ListPendingFn: func(_ context.Context, _ int) ([]*domainModel.OutboxEvent, error) {
			return []*domainModel.OutboxEvent{event}, nil
		},
		MarkPublishedFn: func(_ context.Context, id uint) error {
			if id == event.ID {
				marked = true
			}
			return nil
		},
	}

	// ── relay: single process() tick ─────────────────────────────────────────
	relay := messaging.NewOutboxRelay(repo, producer)
	relay.ProcessOnce(ctx)

	if !marked {
		t.Fatal("expected MarkPublished to be called after successful publish")
	}

	// ── consumer: verify the message landed in Kafka ──────────────────────────
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{testBroker},
		Topic:       topic,
		GroupID:     "test-consumer-" + fmt.Sprint(time.Now().UnixNano()),
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer func() { _ = reader.Close() }()

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if string(msg.Key) != event.AggregateID {
		t.Errorf("key: want %q, got %q", event.AggregateID, string(msg.Key))
	}

	var got domainModel.OutboxEvent
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got.EventID != event.EventID {
		t.Errorf("event_id: want %q, got %q", event.EventID, got.EventID)
	}

	var headerEventType string
	for _, h := range msg.Headers {
		if h.Key == "event_type" {
			headerEventType = string(h.Value)
		}
	}
	if headerEventType != event.EventType {
		t.Errorf("event_type header: want %q, got %q", event.EventType, headerEventType)
	}
}

// TestOutboxRelay_PublishFailure_MarksEventFailed verifies that when the
// publisher returns an error the relay calls MarkFailed instead of MarkPublished.
// This test is broker-independent — it never touches Kafka.
func TestOutboxRelay_PublishFailure_MarksEventFailed(t *testing.T) {
	event := &domainModel.OutboxEvent{
		ID: 7, EventID: "evt-fail", EventType: domainModel.EventTypeTodoUpdated,
		AggregateID: "99", Payload: `{}`, Status: domainModel.OutboxStatusPending,
	}

	markedFailed := false
	repo := &repoMock.OutboxRepositoryMock{
		ListPendingFn: func(_ context.Context, _ int) ([]*domainModel.OutboxEvent, error) {
			return []*domainModel.OutboxEvent{event}, nil
		},
		MarkFailedFn: func(_ context.Context, id uint) error {
			if id == event.ID {
				markedFailed = true
			}
			return nil
		},
	}

	relay := messaging.NewOutboxRelay(repo, &failPublisher{})
	relay.ProcessOnce(context.Background())

	if !markedFailed {
		t.Fatal("expected MarkFailed to be called after publish error")
	}
}

// failPublisher always returns an error from Publish.
type failPublisher struct{}

func (f *failPublisher) Publish(_ context.Context, _ *domainModel.OutboxEvent) error {
	return fmt.Errorf("broker unreachable")
}
func (f *failPublisher) Close() error { return nil }
