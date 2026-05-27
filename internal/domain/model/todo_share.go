package model

import "time"

// Entity: share relationship between a todo and a user
type TodoShare struct {
	TodoID    uint      `db:"todo_id"`
	UserID    int64     `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}
