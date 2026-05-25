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

// createTopic pre-creates a topic via the Kafka admin API.
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

func stubDelivery(deliveryID, eventID uint) *domainModel.OutboxDelivery {
	return &domainModel.OutboxDelivery{
		ID:            deliveryID,
		OutboxEventID: eventID,
		Destination:   domainModel.OutboxDestinationKafka,
		Status:        domainModel.OutboxDeliveryStatusPending,
	}
}

func stubEvent(id uint) *domainModel.OutboxEvent {
	return &domainModel.OutboxEvent{
		ID:            id,
		EventID:       "evt-abc123",
		EventType:     domainModel.EventTypeTodoCreated,
		AggregateType: "todo",
		AggregateID:   "42",
		Payload:       `{"title":"Buy milk"}`,
		CreatedAt:     time.Now(),
	}
}

// TestKafkaOutboxRelay_PublishesToKafka is a real integration test.
// Requires a running Kafka broker at localhost:9092.
func TestKafkaOutboxRelay_PublishesToKafka(t *testing.T) {
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

	delivery := stubDelivery(1, 10)
	event := stubEvent(10)

	marked := false
	repo := &repoMock.OutboxRepositoryMock{
		ListPendingDeliveriesFn: func(_ context.Context, _ string, _ int) ([]*domainModel.OutboxDelivery, error) {
			return []*domainModel.OutboxDelivery{delivery}, nil
		},
		GetEventByIDFn: func(_ context.Context, _ uint) (*domainModel.OutboxEvent, error) {
			return event, nil
		},
		MarkDeliveryPublishedFn: func(_ context.Context, id uint) error {
			if id == delivery.ID {
				marked = true
			}
			return nil
		},
	}

	relay := messaging.NewKafkaOutboxRelay(repo, producer)
	relay.ProcessOnce(ctx)

	if !marked {
		t.Fatal("expected MarkDeliveryPublished to be called after successful publish")
	}

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

// TestKafkaOutboxRelay_PublishFailure_LeavesDeliveryPending verifies that when
// the publisher returns a transient error the relay leaves the delivery as
// pending (no MarkDeliveryFailed call) so it will be retried on the next tick.
// Broker-independent — never touches Kafka.
func TestKafkaOutboxRelay_PublishFailure_LeavesDeliveryPending(t *testing.T) {
	delivery := stubDelivery(7, 20)
	event := stubEvent(20)

	markedFailed := false
	markedPublished := false
	repo := &repoMock.OutboxRepositoryMock{
		ListPendingDeliveriesFn: func(_ context.Context, _ string, _ int) ([]*domainModel.OutboxDelivery, error) {
			return []*domainModel.OutboxDelivery{delivery}, nil
		},
		GetEventByIDFn: func(_ context.Context, _ uint) (*domainModel.OutboxEvent, error) {
			return event, nil
		},
		MarkDeliveryFailedFn: func(_ context.Context, _ uint, _ string) error {
			markedFailed = true
			return nil
		},
		MarkDeliveryPublishedFn: func(_ context.Context, _ uint) error {
			markedPublished = true
			return nil
		},
	}

	relay := messaging.NewKafkaOutboxRelay(repo, &failPublisher{})
	relay.ProcessOnce(context.Background())

	if markedFailed {
		t.Fatal("relay must NOT call MarkDeliveryFailed on transient publish error — delivery should stay pending for retry")
	}
	if markedPublished {
		t.Fatal("relay must NOT call MarkDeliveryPublished when publish failed")
	}
}

// TestRabbitMQOutboxRelay_PublishFailure_LeavesDeliveryPending verifies the same
// retry behaviour for the RabbitMQ relay. Broker-independent.
func TestRabbitMQOutboxRelay_PublishFailure_LeavesDeliveryPending(t *testing.T) {
	delivery := &domainModel.OutboxDelivery{
		ID:            3,
		OutboxEventID: 30,
		Destination:   domainModel.OutboxDestinationRabbitMQ,
		Status:        domainModel.OutboxDeliveryStatusPending,
	}
	event := &domainModel.OutboxEvent{
		ID: 30, EventID: "evt-rmq-fail", EventType: domainModel.EventTypeTodoUpdated,
		AggregateType: "todo", AggregateID: "99", Payload: `{}`,
	}

	markedFailed := false
	markedPublished := false
	repo := &repoMock.OutboxRepositoryMock{
		ListPendingDeliveriesFn: func(_ context.Context, dest string, _ int) ([]*domainModel.OutboxDelivery, error) {
			if dest == domainModel.OutboxDestinationRabbitMQ {
				return []*domainModel.OutboxDelivery{delivery}, nil
			}
			return nil, nil
		},
		GetEventByIDFn: func(_ context.Context, _ uint) (*domainModel.OutboxEvent, error) {
			return event, nil
		},
		MarkDeliveryFailedFn: func(_ context.Context, _ uint, _ string) error {
			markedFailed = true
			return nil
		},
		MarkDeliveryPublishedFn: func(_ context.Context, _ uint) error {
			markedPublished = true
			return nil
		},
	}

	relay := messaging.NewRabbitMQOutboxRelay(repo, &failPublisher{})
	relay.ProcessOnce(context.Background())

	if markedFailed {
		t.Fatal("relay must NOT call MarkDeliveryFailed on transient publish error — delivery should stay pending for retry")
	}
	if markedPublished {
		t.Fatal("relay must NOT call MarkDeliveryPublished when publish failed")
	}
}

// failPublisher always returns an error from Publish.
type failPublisher struct{}

func (f *failPublisher) Publish(_ context.Context, _ *domainModel.OutboxEvent) error {
	return fmt.Errorf("broker unreachable")
}
func (f *failPublisher) Close() error { return nil }
