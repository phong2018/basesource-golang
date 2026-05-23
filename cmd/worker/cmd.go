package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/container"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	"github.com/yourname/go-clean-base/pkg/logger"
	"golang.org/x/sync/errgroup"
)

func NewWorkerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Start outbox relay and event consumer",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Setup()
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			cfg, err := config.NewConfig()
			if err != nil {
				return err
			}

			c, err := container.NewContainer(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() {
				_ = c.DBClient.Close()
				_ = c.RabbitMQClient.Close()
			}()

			g, gCtx := errgroup.WithContext(ctx)

			// outbox relay: polls DB, publishes to RabbitMQ
			g.Go(func() error {
				slog.Info("outbox relay starting")
				c.OutboxRelay.Start(gCtx)
				return nil
			})

			// consumer: processes todo events from RabbitMQ
			g.Go(func() error {
				slog.Info("event consumer starting")
				if err := c.EventConsumer.Start(gCtx, "todo.events.worker", "#", handleEvent); err != nil {
					return err
				}
				<-gCtx.Done()
				return nil
			})

			slog.Info("worker started")
			return g.Wait()
		},
	}
}

func handleEvent(body []byte) error {
	var event domainModel.OutboxEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}
	slog.Info("event received", "event_type", event.EventType, "aggregate_id", event.AggregateID)
	return nil
}
