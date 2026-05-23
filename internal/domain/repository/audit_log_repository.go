package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/domain/model"
)

type IAuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
}
