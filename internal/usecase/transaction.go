package usecase

import "context"

type ITransaction interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
