package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	Database     DatabaseConfig
	AWS          AWSConfig
	Notification NotificationConfig
	Messaging    MessagingConfig
	JWT          JWTConfig
}

type JWTConfig struct {
	Secret           string
	AccessTTLMinutes int
	RefreshTTLDays   int
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
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", ""),
			AccessTTLMinutes: getEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
			RefreshTTLDays:   getEnvInt("JWT_REFRESH_TTL_DAYS", 7),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
