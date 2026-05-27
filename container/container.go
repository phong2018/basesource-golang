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
	infraToken "github.com/yourname/go-clean-base/internal/infrastructure/token"
	workerPresentation "github.com/yourname/go-clean-base/internal/presentation/worker"
	"github.com/yourname/go-clean-base/internal/usecase"
)

// Container holds every top-level dependency that the application needs.
// It is built once at startup and passed to the HTTP server and worker.
// Fields are grouped by infrastructure concern for easy navigation.
type Container struct {
	Cfg      *config.Config
	DBClient *database.Client

	// RabbitMQ — two independent connections: one for publishing (relay), one for consuming
	RabbitMQPublisherClient      *messaging.RabbitMQClient
	RabbitMQConsumerClient       *messaging.RabbitMQClient
	RabbitMQNotificationConsumer *messaging.RabbitMQConsumer

	// Kafka — domain event streaming (durable, replayable audit log)
	KafkaProducer service.IEventPublisher
	KafkaConsumer *messaging.KafkaConsumer

	// Outbox relays — each polls outbox_deliveries for its own destination independently
	KafkaOutboxRelay    *messaging.OutboxRelay
	RabbitMQOutboxRelay *messaging.OutboxRelay

	// Domain event handler — processes Kafka events and publishes downstream tasks
	DomainEventHandler *workerPresentation.DomainEventHandler

	// External services
	NotificationClient service.INotificationClient
	S3Client           service.IFileStorage

	// Usecase layer
	TodoUsecase      usecase.ITodoUsecase
	AuthUsecase      usecase.IAuthUsecase
	TodoOwnedUsecase usecase.ITodoOwnedUsecase
	TokenService     service.ITokenService
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

	// ── RabbitMQ — separate connections for publisher and consumer ───────────
	rmqPub, err := messaging.NewRabbitMQClient(&cfg.Messaging)
	if err != nil {
		return nil, err
	}
	rmqCon, err := messaging.NewRabbitMQClient(&cfg.Messaging)
	if err != nil {
		_ = rmqPub.Close()
		return nil, err
	}
	rmqNotifConsumer := messaging.NewRabbitMQConsumer(rmqCon)
	rmqPublisher     := messaging.NewRabbitMQPublisher(rmqPub)

	// ── Kafka (domain event streaming) ──────────────────────────────────────
	kafkaWriter   := messaging.NewKafkaWriter(&cfg.Messaging)
	kafkaReader   := messaging.NewKafkaReader(&cfg.Messaging)
	kafkaProducer := messaging.NewKafkaProducer(kafkaWriter)
	kafkaConsumer := messaging.NewKafkaConsumer(kafkaReader)

	// ── Outbox relays — independent per broker ───────────────────────────────
	kafkaOutboxRelay    := messaging.NewKafkaOutboxRelay(outboxRepo, kafkaProducer)
	rabbitmqOutboxRelay := messaging.NewRabbitMQOutboxRelay(outboxRepo, rmqPublisher)

	// ── External HTTP services ───────────────────────────────────────────────
	notifier := httpclient.NewNotificationClient(cfg)

	// ── Object storage ───────────────────────────────────────────────────────
	s3 := s3client.NewS3Client(ctx, cfg)

	// ── Domain event handler ─────────────────────────────────────────────────
	domainEventHandler := workerPresentation.NewDomainEventHandler()

	// ── Usecase layer ────────────────────────────────────────────────────────
	todoUsecase := usecase.NewTodoUsecase(
		todoRepo, auditLogRepo, outboxRepo, db, notifier,
	)

	// ── Auth ──────────────────────────────────────────────────────────────────
	tokenSvc         := infraToken.NewJWTTokenService(cfg.JWT.Secret, cfg.JWT.AccessTTLMinutes)
	userRepo         := infraRepo.NewUserRepository(db)
	todoOwnedRepo    := infraRepo.NewTodoOwnedRepository(db)
	todoCommentRepo  := infraRepo.NewTodoCommentRepository(db)
	authUsecase      := usecase.NewAuthUsecase(userRepo, tokenSvc)
	todoOwnedUsecase := usecase.NewTodoOwnedUsecase(todoOwnedRepo, userRepo, todoCommentRepo)

	return &Container{
		Cfg:      cfg,
		DBClient: db,

		RabbitMQPublisherClient:      rmqPub,
		RabbitMQConsumerClient:       rmqCon,
		RabbitMQNotificationConsumer: rmqNotifConsumer,

		KafkaProducer: kafkaProducer,
		KafkaConsumer: kafkaConsumer,

		KafkaOutboxRelay:    kafkaOutboxRelay,
		RabbitMQOutboxRelay: rabbitmqOutboxRelay,

		DomainEventHandler: domainEventHandler,

		NotificationClient: notifier,
		S3Client:           s3,

		TodoUsecase:      todoUsecase,
		AuthUsecase:      authUsecase,
		TodoOwnedUsecase: todoOwnedUsecase,
		TokenService:     tokenSvc,
	}, nil
}
