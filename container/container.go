package container

import (
	"context"

	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/internal/domain/service"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/internal/infrastructure/httpclient"
	infraRepo "github.com/yourname/go-clean-base/internal/infrastructure/repository"
	s3client "github.com/yourname/go-clean-base/internal/infrastructure/s3"
	"github.com/yourname/go-clean-base/internal/usecase"
)

type Container struct {
	Cfg                *config.Config
	DBClient           *database.Client
	NotificationClient service.INotificationClient
	S3Client           service.IFileStorage
	TodoUsecase        usecase.ITodoUsecase
}

func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	db, err := database.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	todoRepo        := infraRepo.NewTodoRepository(db)
	auditLogRepo    := infraRepo.NewAuditLogRepository(db)
	notifier        := httpclient.NewNotificationClient(cfg)
	s3              := s3client.NewS3Client(ctx, cfg)
	todoUsecase     := usecase.NewTodoUsecase(todoRepo, auditLogRepo, db, notifier)

	return &Container{
		Cfg:                cfg,
		DBClient:           db,
		NotificationClient: notifier,
		S3Client:           s3,
		TodoUsecase:        todoUsecase,
	}, nil
}
