package api

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/container"
	apphttp "github.com/yourname/go-clean-base/internal/presentation/http"
	"github.com/yourname/go-clean-base/pkg/logger"
)

func NewAPICommand() *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Start HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Setup()
			ctx := context.Background()

			cfg, err := config.NewConfig()
			if err != nil {
				return err
			}

			c, err := container.NewContainer(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = c.DBClient.Close() }()

			server := apphttp.NewServer(apphttp.Dependencies{
				TodoUsecase: c.TodoUsecase,
			})
			slog.Info("server starting", "port", cfg.AppPort)
			return server.Start(":" + cfg.AppPort)
		},
	}
}
