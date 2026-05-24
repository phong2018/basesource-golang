package service

import (
	"context"
	"io"
	"time"
)

type IFileStorage interface {
	Save(ctx context.Context, key string, body io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}
