package rabbitmq

import (
	"time"

	"github.com/google/uuid"
)

type RabbitMQMessageBase struct {
	// @Description MessageID is a unique identifier for the message (for tracking and duplicate detection)
	// @Example 123e4567-e89b-12d3-a456-426614174000
	MessageID string `json:"message_id" example:"123e4567-e89b-12d3-a456-426614174000"`

	// @Description Action represents the operation performed (created, updated, deleted)
	// @Example created
	Action string `json:"action" example:"created"`

	// @Description EntityType represents the type of entity (provider, vendor, product, etc.)
	// @Example provider
	EntityType string `json:"entity_type" example:"provider"`

	// @Description EntityID is the unique identifier of the entity
	// @Example 123e4567-e89b-12d3-a456-426614174000
	EntityID string `json:"entity_id" example:"123e4567-e89b-12d3-a456-426614174000"`

	// @Description Data contains the entity data (can be ProviderResponse, VendorResponse, etc.)
	// @Example { "id": "123e4567-e89b-12d3-a456-426614174000", "name": "Provider 1" }
	Data any `json:"data"`

	// @Description Timestamp is when the event occurred
	// @Example 2024-01-01T00:00:00Z
	Timestamp time.Time `json:"timestamp" example:"2024-01-01T00:00:00Z"`

	// @Description Version is the message schema version (for future compatibility)
	// @Example 1.0
	Version string `json:"version" example:"1.0"`
}

// NewRabbitMQMessage creates a new RabbitMQ message base with type-safe constants
// It accepts typed constants to ensure type safety and prevent invalid values
func NewRabbitMQMessage(action string, entityType string, entityID string, data any) *RabbitMQMessageBase {
	return &RabbitMQMessageBase{
		MessageID:  uuid.New().String(),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Data:       data,
		Timestamp:  time.Now(),
		Version:    "1.0",
	}
}
