package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	Database     DatabaseConfig
	AWS          AWSConfig
	Notification NotificationConfig
	Messaging    MessagingConfig
}

type DatabaseConfig struct {
	DSN string
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3Bucket        string
}

type NotificationConfig struct {
	BaseURL string
	APIKey  string
}

type MessagingConfig struct {
	// RabbitMQ — notification task queue
	RabbitMQURL           string
	RabbitMQExchange      string
	RabbitMQPrefetchCount int

	// Kafka — domain event streaming
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		AppPort: getEnv("APP_PORT", "8080"),
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_DSN", "root:root@tcp(localhost:3306)/appdb?parseTime=true&charset=utf8mb4"),
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "ap-northeast-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			S3Bucket:        getEnv("AWS_S3_BUCKET", ""),
		},
		Notification: NotificationConfig{
			BaseURL: getEnv("NOTIFICATION_BASE_URL", ""),
			APIKey:  getEnv("NOTIFICATION_API_KEY", ""),
		},
		Messaging: MessagingConfig{
			RabbitMQURL:           getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			RabbitMQExchange:      getEnv("RABBITMQ_EXCHANGE", "todo.events"),
			RabbitMQPrefetchCount: 1,
			KafkaBrokers:          strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			KafkaTopic:            getEnv("KAFKA_TOPIC", "todo-events"),
			KafkaGroupID:          getEnv("KAFKA_GROUP_ID", "todo-worker"),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
