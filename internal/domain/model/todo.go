package model

import "time"

// Entity: Todo represents a task persisted in the database.
type Todo struct {
	ID          uint      `db:"id"`
	Title       string    `db:"title"`
	Description *string   `db:"description"`
	Done        bool      `db:"done"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

