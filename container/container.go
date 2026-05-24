package container

import (
	"context"

	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/internal/infrastructure/httpclient"
	"github.com/yourname/go-clean-base/internal/infrastructure/messaging"
	infraRepo "github.com/yourname/go-clean-base/internal/infrastructure/repository"
	s3client "github.com/yourname/go-clean-base/internal/infrastructure/s3"
	"github.com/yourname/go-clean-base/internal/usecase"
)

// Container holds every top-level dependency that the application needs.
// It is built once at startup and passed to the HTTP server and worker.
// Fields are grouped by infrastructure concern for easy navigation.
type Container struct {
	Cfg      *config.Config
	DBClient *database.Client

	// RabbitMQ — notification task queue (fire-and-forget jobs: email, push)
	RabbitMQClient               *messaging.RabbitMQClient
	RabbitMQNotificationConsumer *messaging.RabbitMQConsumer

	// Kafka — domain event streaming (durable, replayable audit log)
	KafkaProducer service.IEventPublisher
	KafkaConsumer *messaging.KafkaConsumer

	// Outbox relay — polls DB outbox_events and forwards to Kafka
	OutboxRelay *messaging.OutboxRelay

	// External services
	NotificationClient service.INotificationClient
	S3Client           service.IFileStorage

	// Usecase layer
	TodoUsecase usecase.ITodoUsecase
}

func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	// ── Database ────────────────────────────────────────────────────────────
	db, err := database.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	// ── Repositories (depend only on DB) ────────────────────────────────────
	todoRepo     := infraRepo.NewTodoRepository(db)
	auditLogRepo := infraRepo.NewAuditLogRepository(db)
	outboxRepo   := infraRepo.NewOutboxRepository(db)

	// ── RabbitMQ (notification task queue) ──────────────────────────────────
	// Used for fire-and-forget jobs: usecase publishes a task, worker consumes it.
	rmq, err := messaging.NewRabbitMQClient(&cfg.Messaging)
	if err != nil {
		return nil, err
	}
	rmqNotifPublisher := messaging.NewRabbitMQNotificationPublisher(rmq)
	rmqNotifConsumer  := messaging.NewRabbitMQConsumer(rmq)

	// ── Kafka (domain event streaming) ──────────────────────────────────────
	// OutboxRelay reads pending outbox_events from DB and publishes to Kafka.
	// KafkaConsumer reads from Kafka for audit log / analytics downstream.
	kafkaWriter   := messaging.NewKafkaWriter(&cfg.Messaging)
	kafkaReader   := messaging.NewKafkaReader(&cfg.Messaging)
	kafkaProducer := messaging.NewKafkaProducer(kafkaWriter)
	kafkaConsumer := messaging.NewKafkaConsumer(kafkaReader)
	kafkaOutboxRelay := messaging.NewOutboxRelay(outboxRepo, kafkaProducer)

	// ── External HTTP services ───────────────────────────────────────────────
	notifier := httpclient.NewNotificationClient(cfg)

	// ── Object storage ───────────────────────────────────────────────────────
	s3 := s3client.NewS3Client(ctx, cfg)

	// ── Usecase layer ────────────────────────────────────────────────────────
	todoUsecase := usecase.NewTodoUsecase(
		todoRepo, auditLogRepo, outboxRepo, db, notifier, rmqNotifPublisher,
	)

	return &Container{
		Cfg:      cfg,
		DBClient: db,

		RabbitMQClient:               rmq,
		RabbitMQNotificationConsumer: rmqNotifConsumer,

		KafkaProducer: kafkaProducer,
		KafkaConsumer: kafkaConsumer,
		OutboxRelay:   kafkaOutboxRelay,

		NotificationClient: notifier,
		S3Client:           s3,

		TodoUsecase: todoUsecase,
	}, nil
}
