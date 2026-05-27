package model

import "time"

// Entity: comment on a todo
type TodoComment struct {
	ID        uint      `db:"id"`
	TodoID    uint      `db:"todo_id"`
	UserID    int64     `db:"user_id"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
}
