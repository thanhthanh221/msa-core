package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/thanhthanh221/msa-core/pkg/models"
)

// RabbitMQClient interface defines methods for RabbitMQ operations
type RabbitMQClient interface {
	// Publish publishes a message to an exchange
	Publish(ctx context.Context, exchange, routingKey string, message interface{}) error
	// PublishWithOptions publishes a message with custom options
	PublishWithOptions(ctx context.Context, exchange, routingKey string, message interface{}, options models.PublishOptions) error
	// Consume starts consuming messages from a queue
	Consume(ctx context.Context, queue string, handler models.MessageHandler) error
	// ConsumeWithOptions starts consuming messages with custom options
	ConsumeWithOptions(ctx context.Context, queue string, handler models.MessageHandler, options models.ConsumeOptions) error
	// DeclareQueue declares a queue
	DeclareQueue(ctx context.Context, queue string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error
	// DeclareQueueWithDLX declares a queue with Dead Letter Exchange support
	DeclareQueueWithDLX(ctx context.Context, queue string, options models.QueueOptions) error
	// DeclareExchange declares an exchange
	DeclareExchange(ctx context.Context, exchange, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	// DeclareDLX declares a Dead Letter Exchange
	DeclareDLX(ctx context.Context, dlxName string, options models.DLXOptions) error
	// DeclareDLQ declares a Dead Letter Queue and binds it to DLX
	DeclareDLQ(ctx context.Context, dlqName string, dlxName string, options models.DLQOptions) error
	// SetupDLXForQueue sets up DLX/DLQ for an existing queue
	SetupDLXForQueue(ctx context.Context, queueName, dlxName, dlqName string, options models.DLXOptions) error
	// BindQueue binds a queue to an exchange
	BindQueue(ctx context.Context, queue, routingKey, exchange string, noWait bool, args amqp.Table) error
	// Close closes the connection
	Close() error
}

// rabbitmqClient implements RabbitMQClient interface
type rabbitmqClient struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	logger     *logrus.Logger
	tracer     trace.TracerProvider
	propagator propagation.TextMapPropagator
}

// NewRabbitMQClient creates a new RabbitMQ client instance
func NewRabbitMQClient(url string, logger *logrus.Logger, tracer trace.TracerProvider) (RabbitMQClient, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		if logger != nil {
			logger.Errorf("Failed to connect to RabbitMQ: url=%s, error=%s", url, err.Error())
		}
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		if logger != nil {
			logger.Errorf("Failed to open RabbitMQ channel: error=%s", err.Error())
		}
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if logger != nil {
		logger.Info("Successfully connected to RabbitMQ")
	}

	return &rabbitmqClient{
		conn:       conn,
		channel:    channel,
		logger:     logger,
		tracer:     tracer,
		propagator: otel.GetTextMapPropagator(), // W3C Trace Context propagator
	}, nil
}

// trace creates a new span for RabbitMQ operations
func (r *rabbitmqClient) trace(ctx context.Context, operation string) (context.Context, trace.Span) {
	tracer := r.tracer.Tracer("rabbitmq.client")
	return tracer.Start(ctx, fmt.Sprintf("rabbitmq.%s", operation))
}

// Publish publishes a message to an exchange
func (r *rabbitmqClient) Publish(ctx context.Context, exchange, routingKey string, message any) error {
	return r.PublishWithOptions(ctx, exchange, routingKey, message, models.PublishOptions{})
}

// PublishWithOptions publishes a message with custom options
func (r *rabbitmqClient) PublishWithOptions(ctx context.Context, exchange, routingKey string, message interface{}, options models.PublishOptions) error {
	ctx, span := r.trace(ctx, "publish")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.exchange", exchange),
		attribute.String("rabbitmq.routing_key", routingKey),
		attribute.String("rabbitmq.operation", "publish"),
	)

	// Marshal message to JSON if it's not already a byte slice
	var body []byte
	var err error
	switch v := message.(type) {
	case []byte:
		body = v
	case string:
		body = []byte(v)
	default:
		body, err = json.Marshal(message)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if r.logger != nil {
				r.logger.Errorf("Failed to marshal message: operation=publish, exchange=%s, routing_key=%s, error=%s", exchange, routingKey, err.Error())
			}
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}

	// Set default content type if not provided
	contentType := options.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Prepare publishing options
	publishing := amqp.Publishing{
		ContentType:  contentType,
		Body:         body,
		DeliveryMode: amqp.Persistent, // Make message persistent
		Timestamp:    time.Now(),
	}

	// Initialize headers if nil
	if publishing.Headers == nil {
		publishing.Headers = make(amqp.Table)
	}

	// Merge with user-provided headers
	if options.Headers != nil {
		maps.Copy(publishing.Headers, options.Headers)
	}

	// Inject tracing context into message headers for distributed tracing
	carrier := &models.AMQPCarrier{Headers: publishing.Headers}
	r.propagator.Inject(ctx, carrier)
	publishing.Headers = carrier.Headers

	if options.MessageID != "" {
		publishing.MessageId = options.MessageID
		// Also add to headers for easy access in consumer
		if publishing.Headers == nil {
			publishing.Headers = make(amqp.Table)
		}
		publishing.Headers["x-message-id"] = options.MessageID
	}

	// Log publisher trace info for debugging
	if r.logger != nil {
		spanCtx := trace.SpanFromContext(ctx).SpanContext()
		if spanCtx.IsValid() {
			traceparent := carrier.Get("traceparent")
			r.logger.Debugf("Publisher trace context injected: trace_id=%s, span_id=%s, traceparent=%s, exchange=%s, routing_key=%s, message_id=%s",
				spanCtx.TraceID().String(), spanCtx.SpanID().String(), traceparent, exchange, routingKey, publishing.MessageId)
		}
	}

	if options.Priority > 0 {
		publishing.Priority = options.Priority
	}
	if options.Expiration != "" {
		publishing.Expiration = options.Expiration
	}
	if !options.Timestamp.IsZero() {
		publishing.Timestamp = options.Timestamp
	}
	if options.Type != "" {
		publishing.Type = options.Type
	}
	if options.UserID != "" {
		publishing.UserId = options.UserID
	}
	if options.AppID != "" {
		publishing.AppId = options.AppID
	}

	// Publish message
	err = r.channel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		options.Mandatory,
		options.Immediate,
		publishing,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to publish message to RabbitMQ: operation=publish, exchange=%s, routing_key=%s, error=%s", exchange, routingKey, err.Error())
		}
		return fmt.Errorf("failed to publish message: %w", err)
	}

	span.SetAttributes(attribute.Int("rabbitmq.message_size", len(body)))
	span.SetStatus(codes.Ok, "Message published successfully")

	if r.logger != nil {
		r.logger.Debugf("Message published successfully: exchange=%s, routing_key=%s, message_size=%d", exchange, routingKey, len(body))
	}

	return nil
}

// Consume starts consuming messages from a queue
func (r *rabbitmqClient) Consume(ctx context.Context, queue string, handler models.MessageHandler) error {
	return r.ConsumeWithOptions(ctx, queue, handler, models.ConsumeOptions{
		AutoAck: false, // Manual acknowledgment by default
	})
}

// ConsumeWithOptions starts consuming messages with custom options
func (r *rabbitmqClient) ConsumeWithOptions(ctx context.Context, queue string, handler models.MessageHandler, options models.ConsumeOptions) error {
	_, span := r.trace(ctx, "consume")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.String("rabbitmq.operation", "consume"),
		attribute.Bool("rabbitmq.auto_ack", options.AutoAck),
	)

	// Set default consumer tag if not provided
	consumer := options.Consumer
	if consumer == "" {
		consumer = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}

	deliveries, err := r.channel.Consume(
		queue,
		consumer,
		options.AutoAck,
		options.Exclusive,
		options.NoLocal,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to start consuming messages from RabbitMQ: operation=consume, queue=%s, consumer=%s, error=%s", queue, consumer, err.Error())
		}
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	span.SetStatus(codes.Ok, "Started consuming messages")

	if r.logger != nil {
		r.logger.Infof("Started consuming messages from RabbitMQ: queue=%s, consumer=%s", queue, consumer)
	}

	go func() {
		for delivery := range deliveries {
			// Extract publisher trace context from message headers for SpanLink
			// Consumer creates a NEW trace (not continuing publisher trace)
			// Publisher and Consumer traces are linked via SpanLink (async messaging pattern)
			carrier := &models.AMQPCarrier{Headers: delivery.Headers}
			publisherCtx := r.propagator.Extract(context.Background(), carrier)

			// Extract publisher span context for SpanLink
			var publisherSpanCtx trace.SpanContext
			if spanCtx := trace.SpanContextFromContext(publisherCtx); spanCtx.IsValid() {
				publisherSpanCtx = spanCtx
			}

			// Create a NEW trace for consumer (not continuing publisher trace)
			// Start with context.Background() to create independent trace
			tracer := r.tracer.Tracer("rabbitmq.consumer")

			// Create span with SpanLink to publisher trace (if available)
			var spanOptions []trace.SpanStartOption
			if publisherSpanCtx.IsValid() {
				// Create SpanLink to publisher trace
				link := trace.Link{
					SpanContext: publisherSpanCtx,
					Attributes: []attribute.KeyValue{
						attribute.String("messaging.message_id", delivery.MessageId),
						attribute.String("messaging.routing_key", delivery.RoutingKey),
						attribute.String("messaging.exchange", delivery.Exchange),
					},
				}
				spanOptions = append(spanOptions, trace.WithLinks(link))

				if r.logger != nil {
					r.logger.Debugf("Consumer trace linked to publisher: publisher_trace_id=%s, publisher_span_id=%s, message_id=%s, queue=%s",
						publisherSpanCtx.TraceID().String(), publisherSpanCtx.SpanID().String(), delivery.MessageId, queue)
				}
			} else {
				if r.logger != nil {
					r.logger.Debugf("Consumer trace created without link (no publisher trace context): message_id=%s, queue=%s",
						delivery.MessageId, queue)
				}
			}

			// Create new trace for consumer (independent from publisher)
			deliveryCtx, deliverySpan := tracer.Start(context.Background(), "rabbitmq.handle_message", spanOptions...)
			deliverySpan.SetAttributes(
				attribute.String("rabbitmq.queue", queue),
				attribute.String("rabbitmq.message_id", delivery.MessageId),
				attribute.String("rabbitmq.routing_key", delivery.RoutingKey),
				attribute.Int("rabbitmq.message_size", len(delivery.Body)),
			)

			err := handler(deliveryCtx, delivery)
			if err != nil {
				deliverySpan.RecordError(err)
				deliverySpan.SetStatus(codes.Error, err.Error())
				if r.logger != nil {
					r.logger.Errorf("Failed to handle message: operation=handle_message, queue=%s, message_id=%s, error=%s", queue, delivery.MessageId, err.Error())
				}

				// Reject message if not auto-ack
				if !options.AutoAck {
					if err := delivery.Nack(false, true); err != nil {
						if r.logger != nil {
							r.logger.Errorf("Failed to nack message: error=%s", err.Error())
						}
					}
				}
			} else {
				deliverySpan.SetStatus(codes.Ok, "Message handled successfully")
				// Acknowledge message if not auto-ack
				if !options.AutoAck {
					if err := delivery.Ack(false); err != nil {
						if r.logger != nil {
							r.logger.Errorf("Failed to ack message: error=%s", err.Error())
						}
					}
				}
			}

			deliverySpan.End()
		}
	}()

	return nil
}

// DeclareQueue declares a queue
func (r *rabbitmqClient) DeclareQueue(ctx context.Context, queue string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	_, span := r.trace(ctx, "declare_queue")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.Bool("rabbitmq.durable", durable),
		attribute.Bool("rabbitmq.auto_delete", autoDelete),
		attribute.Bool("rabbitmq.exclusive", exclusive),
		attribute.String("rabbitmq.operation", "declare_queue"),
	)

	_, err := r.channel.QueueDeclare(
		queue,
		durable,
		autoDelete,
		exclusive,
		noWait,
		args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to declare queue in RabbitMQ: operation=declare_queue, queue=%s, error=%s", queue, err.Error())
		}
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	span.SetStatus(codes.Ok, "Queue declared successfully")

	if r.logger != nil {
		r.logger.Debugf("Queue declared successfully: queue=%s", queue)
	}

	return nil
}

// DeclareExchange declares an exchange
func (r *rabbitmqClient) DeclareExchange(ctx context.Context, exchange, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	_, span := r.trace(ctx, "declare_exchange")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.exchange", exchange),
		attribute.String("rabbitmq.kind", kind),
		attribute.Bool("rabbitmq.durable", durable),
		attribute.Bool("rabbitmq.auto_delete", autoDelete),
		attribute.Bool("rabbitmq.internal", internal),
		attribute.String("rabbitmq.operation", "declare_exchange"),
	)

	err := r.channel.ExchangeDeclare(
		exchange,
		kind,
		durable,
		autoDelete,
		internal,
		noWait,
		args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to declare exchange in RabbitMQ: operation=declare_exchange, exchange=%s, kind=%s, error=%s", exchange, kind, err.Error())
		}
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	span.SetStatus(codes.Ok, "Exchange declared successfully")

	if r.logger != nil {
		r.logger.Debugf("Exchange declared successfully: exchange=%s, kind=%s", exchange, kind)
	}

	return nil
}

// BindQueue binds a queue to an exchange
func (r *rabbitmqClient) BindQueue(ctx context.Context, queue, routingKey, exchange string, noWait bool, args amqp.Table) error {
	_, span := r.trace(ctx, "bind_queue")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.String("rabbitmq.exchange", exchange),
		attribute.String("rabbitmq.routing_key", routingKey),
		attribute.String("rabbitmq.operation", "bind_queue"),
	)

	err := r.channel.QueueBind(
		queue,
		routingKey,
		exchange,
		noWait,
		args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to bind queue to exchange in RabbitMQ: operation=bind_queue, queue=%s, exchange=%s, routing_key=%s, error=%s", queue, exchange, routingKey, err.Error())
		}
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	span.SetStatus(codes.Ok, "Queue bound successfully")

	if r.logger != nil {
		r.logger.Debugf("Queue bound successfully: queue=%s, exchange=%s, routing_key=%s", queue, exchange, routingKey)
	}

	return nil
}

// DeclareQueueWithDLX declares a queue with Dead Letter Exchange support
func (r *rabbitmqClient) DeclareQueueWithDLX(ctx context.Context, queue string, options models.QueueOptions) error {
	_, span := r.trace(ctx, "declare_queue_with_dlx")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.Bool("rabbitmq.durable", options.Durable),
		attribute.Bool("rabbitmq.auto_delete", options.AutoDelete),
		attribute.String("rabbitmq.operation", "declare_queue_with_dlx"),
	)

	// Build queue arguments
	args := make(amqp.Table)
	if options.AdditionalArgs != nil {
		for k, v := range options.AdditionalArgs {
			args[k] = v
		}
	}

	// Set up Dead Letter Exchange if provided
	if options.DLXName != "" {
		args["x-dead-letter-exchange"] = options.DLXName
		dlxRoutingKey := options.DLXRoutingKey
		if dlxRoutingKey == "" {
			dlxRoutingKey = queue // Default to queue name as routing key
		}
		args["x-dead-letter-routing-key"] = dlxRoutingKey
		span.SetAttributes(
			attribute.String("rabbitmq.dlx", options.DLXName),
			attribute.String("rabbitmq.dlx_routing_key", dlxRoutingKey),
		)
	}

	// Set message TTL if provided
	if options.MessageTTL > 0 {
		args["x-message-ttl"] = options.MessageTTL
		span.SetAttributes(attribute.Int("rabbitmq.message_ttl", options.MessageTTL))
	}

	// Set max queue length if provided
	if options.MaxLength > 0 {
		args["x-max-length"] = options.MaxLength
		span.SetAttributes(attribute.Int("rabbitmq.max_length", options.MaxLength))
	}

	// Set max priority if provided
	if options.MaxPriority > 0 {
		args["x-max-priority"] = options.MaxPriority
		span.SetAttributes(attribute.Int("rabbitmq.max_priority", int(options.MaxPriority)))
	}

	// Set max retries if provided (custom header)
	if options.MaxRetries > 0 {
		if args["x-max-retries"] == nil {
			args["x-max-retries"] = options.MaxRetries
		}
		span.SetAttributes(attribute.Int("rabbitmq.max_retries", options.MaxRetries))
	}

	_, err := r.channel.QueueDeclare(
		queue,
		options.Durable,
		options.AutoDelete,
		options.Exclusive,
		options.NoWait,
		args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to declare queue with DLX in RabbitMQ: operation=declare_queue_with_dlx, queue=%s, dlx=%s, error=%s", queue, options.DLXName, err.Error())
		}
		return fmt.Errorf("failed to declare queue with DLX: %w", err)
	}

	span.SetStatus(codes.Ok, "Queue with DLX declared successfully")

	if r.logger != nil {
		r.logger.Debugf("Queue with DLX declared successfully: queue=%s, dlx=%s", queue, options.DLXName)
	}

	return nil
}

// DeclareDLX declares a Dead Letter Exchange
func (r *rabbitmqClient) DeclareDLX(ctx context.Context, dlxName string, options models.DLXOptions) error {
	_, span := r.trace(ctx, "declare_dlx")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.dlx", dlxName),
		attribute.String("rabbitmq.kind", options.Kind),
		attribute.Bool("rabbitmq.durable", options.Durable),
		attribute.String("rabbitmq.operation", "declare_dlx"),
	)

	// Default to "direct" exchange type if not specified
	kind := options.Kind
	if kind == "" {
		kind = "direct"
	}

	err := r.channel.ExchangeDeclare(
		dlxName,
		kind,
		options.Durable,
		options.AutoDelete,
		options.Internal,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to declare Dead Letter Exchange in RabbitMQ: operation=declare_dlx, dlx=%s, kind=%s, error=%s", dlxName, kind, err.Error())
		}
		return fmt.Errorf("failed to declare DLX: %w", err)
	}

	span.SetStatus(codes.Ok, "Dead Letter Exchange declared successfully")

	if r.logger != nil {
		r.logger.Debugf("Dead Letter Exchange declared successfully: dlx=%s, kind=%s", dlxName, kind)
	}

	return nil
}

// DeclareDLQ declares a Dead Letter Queue and binds it to DLX
func (r *rabbitmqClient) DeclareDLQ(ctx context.Context, dlqName string, dlxName string, options models.DLQOptions) error {
	_, span := r.trace(ctx, "declare_dlq")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.dlq", dlqName),
		attribute.String("rabbitmq.dlx", dlxName),
		attribute.Bool("rabbitmq.durable", options.Durable),
		attribute.String("rabbitmq.operation", "declare_dlq"),
	)

	// Declare the DLQ
	_, err := r.channel.QueueDeclare(
		dlqName,
		options.Durable,
		options.AutoDelete,
		options.Exclusive,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to declare Dead Letter Queue in RabbitMQ: operation=declare_dlq, dlq=%s, error=%s", dlqName, err.Error())
		}
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Bind DLQ to DLX using DLQ name as routing key
	err = r.channel.QueueBind(
		dlqName,
		dlqName, // Use DLQ name as routing key
		dlxName,
		options.NoWait,
		nil,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("Failed to bind Dead Letter Queue to DLX in RabbitMQ: operation=bind_dlq, dlq=%s, dlx=%s, error=%s", dlqName, dlxName, err.Error())
		}
		return fmt.Errorf("failed to bind DLQ to DLX: %w", err)
	}

	span.SetStatus(codes.Ok, "Dead Letter Queue declared and bound successfully")

	if r.logger != nil {
		r.logger.Debugf("Dead Letter Queue declared and bound successfully: dlq=%s, dlx=%s", dlqName, dlxName)
	}

	return nil
}

// SetupDLXForQueue sets up DLX/DLQ for an existing queue
func (r *rabbitmqClient) SetupDLXForQueue(ctx context.Context, queueName, dlxName, dlqName string, options models.DLXOptions) error {
	_, span := r.trace(ctx, "setup_dlx_for_queue")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queueName),
		attribute.String("rabbitmq.dlx", dlxName),
		attribute.String("rabbitmq.dlq", dlqName),
		attribute.String("rabbitmq.operation", "setup_dlx_for_queue"),
	)

	// Step 1: Declare DLX
	if err := r.DeclareDLX(ctx, dlxName, options); err != nil {
		return fmt.Errorf("failed to setup DLX: %w", err)
	}

	// Step 2: Declare DLQ
	dlqOptions := models.DLQOptions{
		Durable:    options.Durable,
		AutoDelete: false, // DLQ should not auto-delete
		Exclusive:  false,
		NoWait:     options.NoWait,
	}
	if err := r.DeclareDLQ(ctx, dlqName, dlxName, dlqOptions); err != nil {
		return fmt.Errorf("failed to setup DLQ: %w", err)
	}

	// Step 3: Update queue to use DLX
	// Note: Queue properties cannot be changed after creation in RabbitMQ
	// This method will attempt to delete and recreate the queue with DLX args
	// WARNING: This will delete all messages in the queue!
	args := make(amqp.Table)
	args["x-dead-letter-exchange"] = dlxName
	args["x-dead-letter-routing-key"] = dlqName

	// Check if queue exists using QueueDeclare with passive mode
	_, err := r.channel.QueueDeclarePassive(queueName, false, false, false, false, nil)
	queueExists := err == nil

	if !queueExists {
		// If queue doesn't exist, just declare it with DLX
		if r.logger != nil {
			r.logger.Infof("Queue does not exist, will create with DLX: queue=%s", queueName)
		}
		// Declare with default durable settings
		_, err = r.channel.QueueDeclare(
			queueName,
			options.Durable,
			false, // Don't auto-delete
			false, // Not exclusive
			false,
			args,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("failed to declare queue with DLX: %w", err)
		}
	} else {
		// Queue exists, need to delete and recreate
		// WARNING: This deletes all messages!
		if r.logger != nil {
			r.logger.Warnf("Queue exists, will delete and recreate with DLX (messages will be lost): queue=%s", queueName)
		}

		// Delete queue (only if empty, set to false to force delete)
		_, err = r.channel.QueueDelete(queueName, false, false, false)
		if err != nil {
			if r.logger != nil {
				r.logger.Warnf("Failed to delete queue, will try to declare with DLX args anyway: queue=%s, error=%s", queueName, err.Error())
			}
		}

		// Recreate with DLX args
		// Use durable=true as default for important queues
		_, err = r.channel.QueueDeclare(
			queueName,
			options.Durable,
			false, // Don't auto-delete
			false, // Not exclusive
			false,
			args,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			if r.logger != nil {
				r.logger.Errorf("Failed to recreate queue with DLX in RabbitMQ: operation=setup_dlx_for_queue, queue=%s, error=%s", queueName, err.Error())
			}
			return fmt.Errorf("failed to recreate queue with DLX: %w", err)
		}
	}

	span.SetStatus(codes.Ok, "DLX/DLQ setup for queue completed successfully")

	if r.logger != nil {
		r.logger.Infof("DLX/DLQ setup for queue completed successfully: queue=%s, dlx=%s, dlq=%s", queueName, dlxName, dlqName)
	}

	return nil
}

// Close closes the connection
func (r *rabbitmqClient) Close() error {
	var errs []error

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close channel: %w", err))
			if r.logger != nil {
				r.logger.Errorf("Failed to close RabbitMQ channel: error=%s", err.Error())
			}
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
			if r.logger != nil {
				r.logger.Errorf("Failed to close RabbitMQ connection: error=%s", err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing RabbitMQ: %v", errs)
	}

	if r.logger != nil {
		r.logger.Info("RabbitMQ connection closed successfully")
	}

	return nil
}
