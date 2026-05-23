package mock

import (
	"context"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
)

type AuditLogRepositoryMock struct {
	CreateFn func(ctx context.Context, log *domainModel.AuditLog) error
}

var _ domainRepo.IAuditLogRepository = (*AuditLogRepositoryMock)(nil)

func (m *AuditLogRepositoryMock) Create(ctx context.Context, log *domainModel.AuditLog) error {
	return m.CreateFn(ctx, log)
}
