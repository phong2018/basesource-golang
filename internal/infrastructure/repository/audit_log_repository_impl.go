package repository

import (
	"context"
	"fmt"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainRepo "github.com/yourname/go-clean-base/internal/domain/repository"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

type auditLogRepository struct {
	baseRepository
}

func NewAuditLogRepository(db *database.Client) domainRepo.IAuditLogRepository {
	return &auditLogRepository{baseRepository{db: db}}
}

func (r *auditLogRepository) Create(ctx context.Context, log *domainModel.AuditLog) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		"INSERT INTO audit_logs (entity, entity_id, action) VALUES (?, ?, ?)",
		log.Entity, log.EntityID, log.Action,
	)
	if err != nil {
		return fmt.Errorf("AuditLog.Create: %w", err)
	}
	return nil
}
