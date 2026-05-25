package model

// Domain Rule Constants: outbox delivery destinations
const (
	OutboxDestinationKafka    = "kafka"
	OutboxDestinationRabbitMQ = "rabbitmq"
)

// Domain Rule Constants: outbox delivery statuses
const (
	OutboxDeliveryStatusPending   = "pending"
	OutboxDeliveryStatusPublished = "published"
	OutboxDeliveryStatusFailed    = "failed"
)
