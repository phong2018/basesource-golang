package model

import "errors"

// Domain Errors: sentinel errors for the Todo entity.
var ErrTodoNotFound = errors.New("todo not found")
