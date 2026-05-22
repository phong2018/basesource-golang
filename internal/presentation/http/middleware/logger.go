package middleware

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/yourname/go-clean-base/pkg/logger"
)

const headerRequestID = "X-Request-ID"

func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()

			requestID := req.Header.Get(headerRequestID)
			if requestID == "" {
				requestID = uuid.NewString()
			}

			ctx := logger.WithRequestID(req.Context(), requestID)
			c.SetRequest(req.WithContext(ctx))

			slog.InfoContext(ctx, "request started",
				"method", req.Method,
				"path", req.URL.Path,
				"request_id", requestID,
			)

			err := next(c)

			slog.InfoContext(ctx, "request completed",
				"method", req.Method,
				"path", req.URL.Path,
				"status", c.Response().Status,
				"latency_ms", time.Since(start).Milliseconds(),
			)
			return err
		}
	}
}
