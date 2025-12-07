package models

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageHandler is a function type for handling consumed messages
type MessageHandler func(ctx context.Context, delivery amqp.Delivery) error

// AMQPCarrier implements propagation.TextMapCarrier for RabbitMQ headers
type AMQPCarrier struct {
	Headers amqp.Table
}

// Get returns the value associated with the passed key.
func (c *AMQPCarrier) Get(key string) string {
	if c.Headers == nil {
		return ""
	}
	if val, ok := c.Headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Set stores the key-value pair.
func (c *AMQPCarrier) Set(key, val string) {
	if c.Headers == nil {
		c.Headers = make(amqp.Table)
	}
	c.Headers[key] = val
}

// Keys lists the keys stored in this carrier.
func (c *AMQPCarrier) Keys() []string {
	if c.Headers == nil {
		return []string{}
	}
	keys := make([]string, 0, len(c.Headers))
	for k := range c.Headers {
		keys = append(keys, k)
	}
	return keys
}

// PublishOptions contains options for publishing messages
type PublishOptions struct {
	Mandatory   bool
	Immediate   bool
	ContentType string
	Headers     amqp.Table
	Priority    uint8
	Expiration  string
	MessageID   string
	Timestamp   time.Time
	Type        string
	UserID      string
	AppID       string
}

// ConsumeOptions contains options for consuming messages
type ConsumeOptions struct {
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}

// QueueOptions contains options for declaring a queue with DLX support
type QueueOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	// DLX configuration
	DLXName        string // Dead Letter Exchange name
	DLXRoutingKey  string // Routing key for DLX (optional, defaults to queue name)
	MaxRetries     int    // Maximum number of retries before sending to DLQ (0 = disabled)
	MessageTTL     int    // Message TTL in milliseconds (0 = disabled)
	MaxLength      int    // Maximum queue length (0 = unlimited)
	MaxPriority    uint8  // Maximum priority (0 = disabled)
	AdditionalArgs amqp.Table
}

// DLXOptions contains options for Dead Letter Exchange
type DLXOptions struct {
	Kind       string // Exchange type: "direct", "topic", "fanout", "headers"
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

// DLQOptions contains options for Dead Letter Queue
type DLQOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}
