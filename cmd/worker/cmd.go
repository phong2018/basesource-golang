package worker

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/container"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	workerPresentation "github.com/yourname/go-clean-base/internal/presentation/worker"
	"github.com/yourname/go-clean-base/pkg/logger"
	"golang.org/x/sync/errgroup"
)

func NewWorkerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Start outbox relays (Kafka + RabbitMQ) and event consumers",
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
				_ = c.RabbitMQPublisherClient.Close()
			_ = c.RabbitMQConsumerClient.Close()
				_ = c.KafkaConsumer.Close()
				_ = c.KafkaProducer.Close()
			}()

			g, gCtx := errgroup.WithContext(ctx)

			// goroutine 1: outbox relay → Kafka
			g.Go(func() error {
				c.KafkaOutboxRelay.Start(gCtx)
				return nil
			})

			// goroutine 2: outbox relay → RabbitMQ
			g.Go(func() error {
				c.RabbitMQOutboxRelay.Start(gCtx)
				return nil
			})

			// goroutine 3: Kafka consumer — analytics / audit log
			g.Go(func() error {
				slog.Info("kafka consumer starting",
					"topic", cfg.Messaging.KafkaTopic,
					"group", cfg.Messaging.KafkaGroupID)
				return c.KafkaConsumer.Start(gCtx, c.DomainEventHandler.Handle)
			})

			// goroutine 4: RabbitMQ notification consumer — binds todo.notifications queue
			// to the todo.events exchange with routing key todo.created so it receives
			// domain events published by RabbitMQOutboxRelay.
			g.Go(func() error {
				slog.Info("rabbitmq notification consumer starting")
				if err := c.RabbitMQNotificationConsumer.Start(gCtx,
					"todo.notifications", domainModel.EventTypeTodoCreated, workerPresentation.HandleNotificationTask,
				); err != nil {
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
