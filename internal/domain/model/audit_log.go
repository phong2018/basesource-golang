package model

import "time"

// Domain Rule Constants: audit action values for AuditLog.
const (
	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
)

// Entity: AuditLog records every write operation on a domain entity.
type AuditLog struct {
	ID        uint      `db:"id"`
	Entity    string    `db:"entity"`
	EntityID  uint      `db:"entity_id"`
	Action    string    `db:"action"`
	CreatedAt time.Time `db:"created_at"`
}
