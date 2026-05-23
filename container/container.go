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

type Container struct {
	Cfg                *config.Config
	DBClient           *database.Client
	RabbitMQClient     *messaging.RabbitMQClient
	NotificationClient service.INotificationClient
	S3Client           service.IFileStorage
	OutboxRelay        *messaging.OutboxRelay
	EventConsumer      *messaging.RabbitMQConsumer
	TodoUsecase        usecase.ITodoUsecase
}

func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	db, err := database.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	rmq, err := messaging.NewRabbitMQClient(&cfg.Messaging)
	if err != nil {
		return nil, err
	}

	todoRepo     := infraRepo.NewTodoRepository(db)
	auditLogRepo := infraRepo.NewAuditLogRepository(db)
	outboxRepo   := infraRepo.NewOutboxRepository(db)
	notifier     := httpclient.NewNotificationClient(cfg)
	s3           := s3client.NewS3Client(ctx, cfg)
	publisher    := messaging.NewPublisher(rmq)
	outboxRelay  := messaging.NewOutboxRelay(outboxRepo, publisher)
	consumer     := messaging.NewRabbitMQConsumer(rmq)
	todoUsecase  := usecase.NewTodoUsecase(todoRepo, auditLogRepo, outboxRepo, db, notifier)

	return &Container{
		Cfg:                cfg,
		DBClient:           db,
		RabbitMQClient:     rmq,
		NotificationClient: notifier,
		S3Client:           s3,
		OutboxRelay:        outboxRelay,
		EventConsumer:      consumer,
		TodoUsecase:        todoUsecase,
	}, nil
}
