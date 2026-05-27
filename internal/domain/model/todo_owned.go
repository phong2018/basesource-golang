package model

import "time"

// Value Object: owned todo with ownership fields for the owner-scoped surface
type OwnedTodo struct {
	ID            uint       `db:"id"`
	OwnerID       *int64     `db:"owner_id"`
	Title         string     `db:"title"`
	Description   *string    `db:"description"`
	Done          bool       `db:"done"`
	DeletedAt     *time.Time `db:"deleted_at"`
	AttachmentURL *string    `db:"attachment_url"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}
