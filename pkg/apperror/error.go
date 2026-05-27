package apperror

import "fmt"

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("code=%d message=%s err=%s", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("code=%d message=%s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func NotFound(message string) *AppError     { return New(404, message, nil) }
func BadRequest(message string) *AppError   { return New(400, message, nil) }
func Internal(err error) *AppError          { return New(500, "internal server error", err) }
func Unauthorized(message string) *AppError { return New(401, message, nil) }
func Conflict(message string) *AppError     { return New(409, message, nil) }
func Forbidden(message string) *AppError    { return New(403, message, nil) }
