package model

import "github.com/yourname/go-clean-base/pkg/apperror"

var (
	ErrInvalidCredentials = apperror.Unauthorized("invalid email or password")
	ErrEmailTaken         = apperror.Conflict("email already registered")
	ErrTokenExpired       = apperror.Unauthorized("token expired")
	ErrTokenInvalid       = apperror.Unauthorized("token invalid")
)
