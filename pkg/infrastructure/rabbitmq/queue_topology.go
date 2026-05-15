package rabbitmq

import (
	"context"
	"fmt"
)

type QueueTopology struct {
	Name           string
	Exchange       string
	Queue          string
	BindingPattern string
	DLX            string
	DLQ            string
	QueueOptions   QueueOptions
}

// NewQueueTopology returns a QueueTopology with durable defaults for use with SetupQueueTopology.
func NewQueueTopology(queue, exchange, bindingPattern, dlx, dlq string) QueueTopology {
	return QueueTopology{
		Name:           queue,
		Exchange:       exchange,
		Queue:          queue,
		BindingPattern: bindingPattern,
		DLX:            dlx,
		DLQ:            dlq,
		QueueOptions: QueueOptions{
			Durable:        true,
			AutoDelete:     false,
			Exclusive:      false,
			NoWait:         false,
			MaxRetries:     0,
			MessageTTL:     0,
			MaxLength:      0,
			MaxPriority:    0,
			AdditionalArgs: nil,
		},
	}
}

// SetupQueueTopology declares final DLX/DLQ (for explicit poison-message publish), the main queue, and binding.
// Retries are handled in the consumer by republishing to the same exchange/routing key with x-retry-count (no TTL retry queue).
func SetupQueueTopology(client RabbitMQClient, t QueueTopology) error {
	ctx := context.Background()

	if err := client.DeclareDLX(ctx, t.DLX, DLXOptions{
		Kind:       "direct",
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
		Args:       nil,
	}); err != nil {
		return fmt.Errorf("failed to declare DLX: %w", err)
	}

	if err := client.DeclareDLQ(ctx, t.DLQ, t.DLX, DLQOptions{
		Durable:    true,
		AutoDelete: false,
		Exclusive:  false,
		NoWait:     false,
		Args:       nil,
	}); err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	if err := client.DeclareQueue(ctx, t.Queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare main queue: %w", err)
	}

	if err := client.BindQueue(ctx, t.Queue, t.BindingPattern, t.Exchange, false, nil); err != nil {
		return fmt.Errorf("failed to bind queue to exchange: %w", err)
	}

	return nil
}
