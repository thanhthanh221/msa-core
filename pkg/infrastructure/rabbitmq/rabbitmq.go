package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type MessageHandler func(ctx context.Context, delivery amqp091.Delivery) error

// AMQPCarrier implements propagation.TextMapCarrier for RabbitMQ headers
type AMQPCarrier struct {
	Headers amqp091.Table
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
		c.Headers = make(amqp091.Table)
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
	Headers     amqp091.Table
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
	Args      amqp091.Table
	// Retry configuration
	// MaxRetries: max failed processing attempts before Nack(requeue=false) → DLX/DLQ.
	// Retries are tracked via x-retry-count on republished copies (Nack(requeue=true) does not hit DLX, so x-death never increments).
	MaxRetries int // 0 = disabled: on failure always Nack(requeue=true) until success or operator intervention
	// RetryWithDLXTTL enables the "retry queue + TTL + DLX back" pattern.
	// When enabled and MaxRetries > 0:
	// - on handler error and retry not exceeded: Nack(requeue=false) so RabbitMQ dead-letters to the configured retry queue (x-death will increment)
	// - when retry exceeded: publish to DLQ (DLQName or "<queue>.dlq") and Ack original
	RetryWithDLXTTL bool
	// DLQName is used when RetryWithDLXTTL=true and MaxRetries exceeded.
	// If empty, defaults to "<queue>.dlq".
	DLQName string
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
	AdditionalArgs amqp091.Table
}

// DLXOptions contains options for Dead Letter Exchange
type DLXOptions struct {
	Kind       string // Exchange type: "direct", "topic", "fanout", "headers"
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp091.Table
}

// DLQOptions contains options for Dead Letter Queue
type DLQOptions struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp091.Table
}

// RetryTTLTopologyOptions configures a standard RabbitMQ retry setup using retry queue + TTL.
//
// Pattern:
// - main queue dead-letters to retry exchange/queue on Nack(requeue=false)
// - retry queue has x-message-ttl and dead-letters back to main queue (default exchange)
// - DLQ exists for final failure handling (consumer can publish there when retries exceeded)
type RetryTTLTopologyOptions struct {
	Durable        bool
	RetryTTL       time.Duration // e.g. 10s, 30s
	MaxPriority    uint8
	AdditionalArgs amqp091.Table
	// Naming overrides (optional)
	RetryExchange string // default: "<queue>.retry.dlx"
	RetryQueue    string // default: "<queue>.retry"
	DLQName       string // default: "<queue>.dlq"
	DLXName       string // default: "<queue>.dlx"
}

// RabbitMQClient interface defines methods for RabbitMQ operations
type RabbitMQClient interface {
	// Publish publishes a message to an exchange
	Publish(ctx context.Context, exchange, routingKey string, message any) error
	// PublishWithOptions publishes a message with custom options
	PublishWithOptions(ctx context.Context, exchange, routingKey string, message any, options PublishOptions) error
	// Consume starts consuming messages from a queue
	Consume(ctx context.Context, queue string, handler MessageHandler) error
	// ConsumeWithOptions starts consuming messages with custom options
	ConsumeWithOptions(ctx context.Context, queue string, handler MessageHandler, options ConsumeOptions) error
	// DeclareQueue declares a queue
	DeclareQueue(ctx context.Context, queue string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) error
	// DeclareQueueWithDLX declares a queue with Dead Letter Exchange support
	DeclareQueueWithDLX(ctx context.Context, queue string, options QueueOptions) error
	// DeclareExchange declares an exchange
	DeclareExchange(ctx context.Context, exchange, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error
	// DeclareDLX declares a Dead Letter Exchange
	DeclareDLX(ctx context.Context, dlxName string, options DLXOptions) error
	// DeclareDLQ declares a Dead Letter Queue and binds it to DLX
	DeclareDLQ(ctx context.Context, dlqName string, dlxName string, options DLQOptions) error
	// SetupDLXForQueue sets up DLX/DLQ for an existing queue
	SetupDLXForQueue(ctx context.Context, queueName, dlxName, dlqName string, options DLXOptions) error
	// BindQueue binds a queue to an exchange
	BindQueue(ctx context.Context, queue, routingKey, exchange string, noWait bool, args amqp091.Table) error
	// Close closes the connection
	Close() error
}

// SetupRetryTTLTopology declares a queue topology for DLX/TTL retries.
//
// It does NOT change consume behavior by itself. To actually use x-death-based retries,
// consume with ConsumeOptions{MaxRetries: N, RetryWithDLXTTL: true, DLQName: "<queue>.dlq"}.
func SetupRetryTTLTopology(ctx context.Context, client RabbitMQClient, queue string, opt RetryTTLTopologyOptions) error {
	if opt.Durable == false {
		// default durable for retry topology
		opt.Durable = true
	}
	if opt.RetryTTL <= 0 {
		opt.RetryTTL = 10 * time.Second
	}
	retryExchange := opt.RetryExchange
	if retryExchange == "" {
		retryExchange = queue + ".retry.dlx"
	}
	retryQueue := opt.RetryQueue
	if retryQueue == "" {
		retryQueue = queue + ".retry"
	}
	dlxName := opt.DLXName
	if dlxName == "" {
		dlxName = queue + ".dlx"
	}
	dlqName := opt.DLQName
	if dlqName == "" {
		dlqName = queue + ".dlq"
	}

	// Retry exchange and retry queue (TTL then dead-letter back to main queue via default exchange).
	if err := client.DeclareExchange(ctx, retryExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	retryArgs := amqp091.Table{
		"x-message-ttl":             int(opt.RetryTTL.Milliseconds()),
		"x-dead-letter-exchange":    "",    // default exchange
		"x-dead-letter-routing-key": queue, // route back to main queue
	}
	if opt.MaxPriority > 0 {
		retryArgs["x-max-priority"] = opt.MaxPriority
	}
	if opt.AdditionalArgs != nil {
		for k, v := range opt.AdditionalArgs {
			retryArgs[k] = v
		}
	}
	if err := client.DeclareQueue(ctx, retryQueue, opt.Durable, false, false, false, retryArgs); err != nil {
		return err
	}
	if err := client.BindQueue(ctx, retryQueue, retryQueue, retryExchange, false, nil); err != nil {
		return err
	}

	// Main queue: dead-letter to retry exchange/queue.
	if err := client.DeclareQueueWithDLX(ctx, queue, QueueOptions{
		Durable:        opt.Durable,
		AutoDelete:     false,
		Exclusive:      false,
		NoWait:         false,
		DLXName:        retryExchange,
		DLXRoutingKey:  retryQueue,
		MaxPriority:    opt.MaxPriority,
		AdditionalArgs: opt.AdditionalArgs,
	}); err != nil {
		return err
	}

	// DLQ: used when retries exceeded (consumer publishes here).
	if err := client.DeclareDLX(ctx, dlxName, DLXOptions{Kind: "direct", Durable: true}); err != nil {
		return err
	}
	if err := client.DeclareDLQ(ctx, dlqName, dlxName, DLQOptions{Durable: true}); err != nil {
		return err
	}
	if err := client.BindQueue(ctx, dlqName, dlqName, dlxName, false, nil); err != nil {
		return err
	}

	return nil
}

// rabbitmqClient implements RabbitMQClient interface
type rabbitmqClient struct {
	url         string
	conn        *amqp091.Connection
	channel     *amqp091.Channel
	mu          sync.RWMutex // protects conn/channel swap on reconnect
	reconnectMu sync.Mutex   // ensures only one reconnect attempt at a time
	publishMu   sync.Mutex   // serialize channel use (amqp channel is not goroutine-safe)
	logger      *logrus.Logger
	tracer      trace.TracerProvider
	propagator  propagation.TextMapPropagator

	topoMu            sync.RWMutex
	declaredExchanges []exchangeDecl
	declaredQueues    []queueDecl
	queueBindings     []queueBindDecl
}

const (
	defaultPublishTimeout  = 3 * time.Second
	defaultOperationTimout = 5 * time.Second
)

type exchangeDecl struct {
	name       string
	kind       string
	durable    bool
	autoDelete bool
	internal   bool
	noWait     bool
	args       amqp091.Table
}

type queueDecl struct {
	name       string
	durable    bool
	autoDelete bool
	exclusive  bool
	noWait     bool
	args       amqp091.Table
}

type queueBindDecl struct {
	queue      string
	routingKey string
	exchange   string
	noWait     bool
	args       amqp091.Table
}

// NewRabbitMQClient creates a new RabbitMQ client instance
func NewRabbitMQClient(url string, logger *logrus.Logger, tracer trace.TracerProvider) (RabbitMQClient, error) {
	conn, err := dial(url)
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
		url:        url,
		conn:       conn,
		channel:    channel,
		logger:     logger,
		tracer:     tracer,
		propagator: otel.GetTextMapPropagator(), // W3C Trace Context propagator
	}, nil
}

func dial(url string) (*amqp091.Connection, error) {
	// Keep TCP hangs bounded when broker is down/restarting.
	// Heartbeat helps detect half-open connections.
	cfg := amqp091.Config{
		Heartbeat: 10 * time.Second,
		Dial: func(network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
			return d.Dial(network, addr)
		},
	}
	return amqp091.DialConfig(url, cfg)
}

func cloneTable(t amqp091.Table) amqp091.Table {
	if t == nil {
		return nil
	}
	out := make(amqp091.Table, len(t))
	maps.Copy(out, t)
	return out
}

func (r *rabbitmqClient) ensureChannel(ctx context.Context) (*amqp091.Channel, error) {
	// Fast path: channel exists and not closed.
	r.mu.RLock()
	ch := r.channel
	conn := r.conn
	r.mu.RUnlock()

	if ch != nil && !ch.IsClosed() && conn != nil && !conn.IsClosed() {
		return ch, nil
	}

	// Reconnect path (single writer).
	r.reconnectMu.Lock()
	defer r.reconnectMu.Unlock()

	// Re-check after acquiring reconnect lock.
	r.mu.RLock()
	ch = r.channel
	conn = r.conn
	r.mu.RUnlock()
	if ch != nil && !ch.IsClosed() && conn != nil && !conn.IsClosed() {
		return ch, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultOperationTimout)
		defer cancel()
	}

	backoff := 100 * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("rabbitmq reconnect timeout: %w", ctx.Err())
		default:
		}

		newConn, err := dial(r.url)
		if err != nil {
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
			}
			continue
		}

		newCh, err := newConn.Channel()
		if err != nil {
			_ = newConn.Close()
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
			}
			continue
		}

		// Swap in new conn/channel.
		r.mu.Lock()
		oldCh := r.channel
		oldConn := r.conn
		r.conn = newConn
		r.channel = newCh
		r.mu.Unlock()

		if oldCh != nil {
			_ = oldCh.Close()
		}
		if oldConn != nil {
			_ = oldConn.Close()
		}

		if r.logger != nil {
			r.logger.Warn("RabbitMQ reconnected successfully")
		}

		// Best-effort: re-apply topology after reconnect.
		// Do not fail reconnect if redeclare fails.
		redeclareCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.redeclareTopology(redeclareCtx, newCh)
		cancel()

		return newCh, nil
	}
}

func (r *rabbitmqClient) redeclareTopology(ctx context.Context, ch *amqp091.Channel) error {
	if ch == nil {
		return nil
	}

	r.topoMu.RLock()
	exchanges := append([]exchangeDecl(nil), r.declaredExchanges...)
	queues := append([]queueDecl(nil), r.declaredQueues...)
	binds := append([]queueBindDecl(nil), r.queueBindings...)
	r.topoMu.RUnlock()

	for _, ex := range exchanges {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := ch.ExchangeDeclare(ex.name, ex.kind, ex.durable, ex.autoDelete, ex.internal, ex.noWait, ex.args); err != nil {
			if r.logger != nil {
				r.logger.Warnf("RabbitMQ redeclare exchange failed: exchange=%s, error=%s", ex.name, err.Error())
			}
		}
	}

	for _, q := range queues {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := ch.QueueDeclare(q.name, q.durable, q.autoDelete, q.exclusive, q.noWait, q.args); err != nil {
			if r.logger != nil {
				r.logger.Warnf("RabbitMQ redeclare queue failed: queue=%s, error=%s", q.name, err.Error())
			}
		}
	}

	for _, b := range binds {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := ch.QueueBind(b.queue, b.routingKey, b.exchange, b.noWait, b.args); err != nil {
			if r.logger != nil {
				r.logger.Warnf("RabbitMQ rebind queue failed: queue=%s, exchange=%s, routing_key=%s, error=%s", b.queue, b.exchange, b.routingKey, err.Error())
			}
		}
	}

	if r.logger != nil && (len(exchanges) > 0 || len(queues) > 0 || len(binds) > 0) {
		r.logger.Infof("RabbitMQ topology re-applied: exchanges=%d, queues=%d, bindings=%d", len(exchanges), len(queues), len(binds))
	}
	return nil
}

func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), d)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// trace creates a new span for RabbitMQ operations
func (r *rabbitmqClient) trace(ctx context.Context, operation string) (context.Context, trace.Span) {
	tracer := r.tracer.Tracer("rabbitmq.client")
	return tracer.Start(ctx, fmt.Sprintf("rabbitmq.%s", operation))
}

// Publish publishes a message to an exchange
func (r *rabbitmqClient) Publish(ctx context.Context, exchange, routingKey string, message any) error {
	return r.PublishWithOptions(ctx, exchange, routingKey, message, PublishOptions{})
}

// PublishWithOptions publishes a message with custom options
func (r *rabbitmqClient) PublishWithOptions(ctx context.Context, exchange, routingKey string, message any, options PublishOptions) error {
	// IMPORTANT: Create publish span first, but when injecting trace context into headers,
	// we use the original ctx which contains the parent trace context from business logic.
	// This ensures consumer continues the same trace from business logic layer.
	_, span := r.trace(ctx, "publish")
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
	publishing := amqp091.Publishing{
		ContentType:  contentType,
		Body:         body,
		DeliveryMode: amqp091.Persistent, // Make message persistent
		Timestamp:    time.Now(),
	}

	// Initialize headers if nil
	if publishing.Headers == nil {
		publishing.Headers = make(amqp091.Table)
	}

	// Merge with user-provided headers
	if options.Headers != nil {
		maps.Copy(publishing.Headers, options.Headers)
	}

	// Inject publisher trace context into message headers for SpanLink
	// This is used to link consumer trace with publisher trace (async messaging pattern)
	// Publisher and Consumer will have separate traces, linked via SpanLink
	carrier := &AMQPCarrier{Headers: publishing.Headers}
	r.propagator.Inject(ctx, carrier)
	publishing.Headers = carrier.Headers

	// Ensure message-id is set (for linking traces)
	if options.MessageID != "" {
		publishing.MessageId = options.MessageID
		// Also add to headers for easy access in consumer
		if publishing.Headers == nil {
			publishing.Headers = make(amqp091.Table)
		}
		publishing.Headers["x-message-id"] = options.MessageID
	}

	// Log publisher trace info for debugging
	if r.logger != nil {
		spanCtx := trace.SpanFromContext(ctx).SpanContext()
		if spanCtx.IsValid() {
			traceparent := carrier.Get("traceparent")
			r.logger.Debugf("📤 Publisher trace context injected: trace_id=%s, span_id=%s, traceparent=%s, exchange=%s, routing_key=%s, message_id=%s",
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

	// Ensure we never block indefinitely when broker is down/restarting.
	publishCtx, cancel := withDefaultTimeout(ctx, defaultPublishTimeout)
	defer cancel()

	ch, err := r.ensureChannel(publishCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if r.logger != nil {
			r.logger.Errorf("RabbitMQ channel not available: operation=publish, exchange=%s, routing_key=%s, error=%s", exchange, routingKey, err.Error())
		}
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	// Publish message
	r.publishMu.Lock()
	err = ch.PublishWithContext(
		publishCtx,
		exchange,
		routingKey,
		options.Mandatory,
		options.Immediate,
		publishing,
	)
	r.publishMu.Unlock()
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
func (r *rabbitmqClient) Consume(ctx context.Context, queue string, handler MessageHandler) error {
	return r.ConsumeWithOptions(ctx, queue, handler, ConsumeOptions{
		AutoAck: false, // Manual acknowledgment by default
	})
}

// ConsumeWithOptions starts consuming messages with custom options
func (r *rabbitmqClient) ConsumeWithOptions(ctx context.Context, queue string, handler MessageHandler, options ConsumeOptions) error {
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

	span.SetStatus(codes.Ok, "Consume loop started")

	// Process messages in a goroutine with auto-resubscribe on reconnect.
	go func() {
		backoff := 200 * time.Millisecond
		for {
			if ctx != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}

			opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
			_, err := r.ensureChannel(opCtx) // ensures conn is up (and topology re-applied)
			cancel()
			if err != nil {
				if r.logger != nil {
					r.logger.Errorf("RabbitMQ consume: channel not available, will retry: queue=%s, consumer=%s, error=%s", queue, consumer, err.Error())
				}
				time.Sleep(backoff)
				if backoff < 2*time.Second {
					backoff *= 2
				}
				continue
			}

			// Dedicated channel for consuming (separate from shared publish channel).
			r.mu.RLock()
			conn := r.conn
			r.mu.RUnlock()
			if conn == nil || conn.IsClosed() {
				time.Sleep(backoff)
				continue
			}

			consumeCh, err := conn.Channel()
			if err != nil {
				time.Sleep(backoff)
				continue
			}

			deliveries, err := consumeCh.Consume(
				queue,
				consumer,
				options.AutoAck,
				options.Exclusive,
				options.NoLocal,
				options.NoWait,
				options.Args,
			)
			if err != nil {
				_ = consumeCh.Close()
				if r.logger != nil {
					r.logger.Errorf("Failed to start consuming, will retry: queue=%s, consumer=%s, error=%s", queue, consumer, err.Error())
				}
				time.Sleep(backoff)
				if backoff < 2*time.Second {
					backoff *= 2
				}
				continue
			}

			if r.logger != nil {
				r.logger.Infof("Consuming started: queue=%s, consumer=%s", queue, consumer)
			}

			// Reset backoff after a successful subscribe.
			backoff = 200 * time.Millisecond

			for delivery := range deliveries {
				// Extract publisher trace context from message headers for SpanLink
				// Consumer creates a NEW trace (not continuing publisher trace)
				// Publisher and Consumer traces are linked via SpanLink (async messaging pattern)
				carrier := &AMQPCarrier{Headers: delivery.Headers}
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
						r.logger.Debugf("🔗 Consumer trace linked to publisher: publisher_trace_id=%s, publisher_span_id=%s, message_id=%s, queue=%s",
							publisherSpanCtx.TraceID().String(), publisherSpanCtx.SpanID().String(), delivery.MessageId, queue)
					}
				} else {
					if r.logger != nil {
						r.logger.Debugf("📥 Consumer trace created without link (no publisher trace context): message_id=%s, queue=%s",
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
						// Handle retry logic if MaxRetries is configured
						shouldRequeue := true
						retryRepublished := false
						if options.MaxRetries > 0 {
							retryCount := r.getRetryCount(delivery)
							deliverySpan.SetAttributes(attribute.Int("rabbitmq.retry_count", retryCount))

							if options.RetryWithDLXTTL {
								// Count dead-letter cycles based on x-death for this queue.
								// This only increases when we Nack(requeue=false) and broker dead-letters.
								retryCount = r.getXDeathCountForQueue(delivery, queue)
								deliverySpan.SetAttributes(attribute.Int("rabbitmq.retry_count", retryCount))
							}

							if retryCount >= options.MaxRetries {
								// Max retries exceeded, reject without requeue to send to DLQ
								shouldRequeue = false
								if r.logger != nil {
									r.logger.Warnf("Message exceeded max retries: queue=%s, message_id=%s, retry_count=%d, max_retries=%d, sending to DLQ",
										queue, delivery.MessageId, retryCount, options.MaxRetries)
								}
								deliverySpan.SetAttributes(
									attribute.Bool("rabbitmq.sent_to_dlq", true),
									attribute.Int("rabbitmq.max_retries_exceeded", retryCount),
								)
							} else {
								if options.RetryWithDLXTTL {
									// DLX/TTL retry pattern: do NOT requeue.
									// Nack(requeue=false) will dead-letter to retry queue (configured by queue args),
									// then after TTL it will dead-letter back to main queue, incrementing x-death.
									next := retryCount + 1
									if r.logger != nil {
										r.logger.Warnf("Message will be retried via DLX/TTL: queue=%s, message_id=%s, retry_count=%d/%d",
											queue, delivery.MessageId, next, options.MaxRetries)
									}
									deliverySpan.SetAttributes(attribute.Int("rabbitmq.will_retry", next))
								} else {
									// Republish with incremented x-retry-count then Ack the original.
									// Nack(requeue=true) never dead-letters, so x-death-based counting would stay at 0 forever.
									next := retryCount + 1
									if r.logger != nil {
										r.logger.Warnf("Message will be retried: queue=%s, message_id=%s, retry_count=%d/%d",
											queue, delivery.MessageId, next, options.MaxRetries)
									}
									deliverySpan.SetAttributes(attribute.Int("rabbitmq.will_retry", next))

									pubOpts := PublishOptions{
										ContentType: delivery.ContentType,
										MessageID:   delivery.MessageId,
										Headers:     cloneHeadersWithRetryCount(delivery.Headers, next),
										Priority:    delivery.Priority,
										Expiration:  delivery.Expiration,
									}
									exchange := delivery.Exchange
									routingKey := delivery.RoutingKey
									if routingKey == "" {
										routingKey = queue
									}
									pubErr := r.PublishWithOptions(deliveryCtx, exchange, routingKey, delivery.Body, pubOpts)
									if pubErr != nil {
										deliverySpan.RecordError(pubErr)
										if r.logger != nil {
											r.logger.Errorf("Failed to republish message for retry: queue=%s, message_id=%s, error=%s", queue, delivery.MessageId, pubErr.Error())
										}
										if err := delivery.Nack(false, false); err != nil && r.logger != nil {
											r.logger.Errorf("Failed to nack message (no requeue): error=%s", err.Error())
										}
									} else {
										if err := delivery.Ack(false); err != nil && r.logger != nil {
											r.logger.Errorf("Failed to ack message after republish: error=%s", err.Error())
										}
									}
									retryRepublished = true
								}
							}
						}

						if !retryRepublished {
							if shouldRequeue {
								if options.MaxRetries > 0 && options.RetryWithDLXTTL {
									// Retry via DLX/TTL (dead-letter).
									if err := delivery.Nack(false, false); err != nil {
										if r.logger != nil {
											r.logger.Errorf("Failed to nack message (no requeue): error=%s", err.Error())
										}
									}
								} else {
									// Requeue message (will be retried; MaxRetries disabled)
									if err := delivery.Nack(false, true); err != nil {
										if r.logger != nil {
											r.logger.Errorf("Failed to nack message: error=%s", err.Error())
										}
									}
								}
							} else {
								if options.MaxRetries > 0 && options.RetryWithDLXTTL {
									// Exceeded retries: publish to DLQ then Ack original so it doesn't loop via retry DLX.
									dlqName := options.DLQName
									if dlqName == "" {
										dlqName = queue + ".dlq"
									}
									pubErr := r.PublishWithOptions(deliveryCtx, "", dlqName, delivery.Body, PublishOptions{
										ContentType: delivery.ContentType,
										MessageID:   delivery.MessageId,
										Headers:     maps.Clone(delivery.Headers),
										Priority:    delivery.Priority,
										Expiration:  delivery.Expiration,
									})
									if pubErr != nil {
										deliverySpan.RecordError(pubErr)
										if r.logger != nil {
											r.logger.Errorf("Failed to publish message to DLQ: queue=%s, dlq=%s, message_id=%s, error=%s", queue, dlqName, delivery.MessageId, pubErr.Error())
										}
										// Best-effort: reject to DLX (may go to retry DLX depending on queue args).
										if err := delivery.Nack(false, false); err != nil && r.logger != nil {
											r.logger.Errorf("Failed to nack message (no requeue): error=%s", err.Error())
										}
									} else {
										if err := delivery.Ack(false); err != nil && r.logger != nil {
											r.logger.Errorf("Failed to ack message after publishing to DLQ: error=%s", err.Error())
										}
									}
								} else {
									// Reject without requeue (will be sent to DLQ if configured)
									if err := delivery.Nack(false, false); err != nil {
										if r.logger != nil {
											r.logger.Errorf("Failed to nack message (no requeue): error=%s", err.Error())
										}
									}
								}
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

			// deliveries closed: broker restart / channel closed / network hiccup.
			_ = consumeCh.Close()
			if r.logger != nil {
				r.logger.Warnf("RabbitMQ deliveries closed, resubscribing: queue=%s, consumer=%s", queue, consumer)
			}
		}
	}()

	return nil
}

// DeclareQueue declares a queue
func (r *rabbitmqClient) DeclareQueue(ctx context.Context, queue string, durable, autoDelete, exclusive, noWait bool, args amqp091.Table) error {
	_, span := r.trace(ctx, "declare_queue")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.Bool("rabbitmq.durable", durable),
		attribute.Bool("rabbitmq.auto_delete", autoDelete),
		attribute.Bool("rabbitmq.exclusive", exclusive),
		attribute.String("rabbitmq.operation", "declare_queue"),
	)

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	// Record for re-apply after reconnect (best-effort).
	r.topoMu.Lock()
	r.declaredQueues = append(r.declaredQueues, queueDecl{
		name:       queue,
		durable:    durable,
		autoDelete: autoDelete,
		exclusive:  exclusive,
		noWait:     noWait,
		args:       cloneTable(args),
	})
	r.topoMu.Unlock()

	_, err = ch.QueueDeclare(
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
func (r *rabbitmqClient) DeclareExchange(ctx context.Context, exchange, kind string, durable, autoDelete, internal, noWait bool, args amqp091.Table) error {
	// Create span with exchange name for better trace visibility
	spanName := fmt.Sprintf("declare_exchange.%s", exchange)
	_, span := r.trace(ctx, spanName)
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.exchange", exchange),
		attribute.String("rabbitmq.kind", kind),
		attribute.Bool("rabbitmq.durable", durable),
		attribute.Bool("rabbitmq.auto_delete", autoDelete),
		attribute.Bool("rabbitmq.internal", internal),
		attribute.String("rabbitmq.operation", "declare_exchange"),
	)

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	// Record for re-apply after reconnect (best-effort).
	r.topoMu.Lock()
	r.declaredExchanges = append(r.declaredExchanges, exchangeDecl{
		name:       exchange,
		kind:       kind,
		durable:    durable,
		autoDelete: autoDelete,
		internal:   internal,
		noWait:     noWait,
		args:       cloneTable(args),
	})
	r.topoMu.Unlock()

	err = ch.ExchangeDeclare(
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
func (r *rabbitmqClient) BindQueue(ctx context.Context, queue, routingKey, exchange string, noWait bool, args amqp091.Table) error {
	_, span := r.trace(ctx, "bind_queue")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.String("rabbitmq.exchange", exchange),
		attribute.String("rabbitmq.routing_key", routingKey),
		attribute.String("rabbitmq.operation", "bind_queue"),
	)

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	// Record for re-apply after reconnect (best-effort).
	r.topoMu.Lock()
	r.queueBindings = append(r.queueBindings, queueBindDecl{
		queue:      queue,
		routingKey: routingKey,
		exchange:   exchange,
		noWait:     noWait,
		args:       cloneTable(args),
	})
	r.topoMu.Unlock()

	err = ch.QueueBind(
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
func (r *rabbitmqClient) DeclareQueueWithDLX(ctx context.Context, queue string, options QueueOptions) error {
	_, span := r.trace(ctx, "declare_queue_with_dlx")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.queue", queue),
		attribute.Bool("rabbitmq.durable", options.Durable),
		attribute.Bool("rabbitmq.auto_delete", options.AutoDelete),
		attribute.String("rabbitmq.operation", "declare_queue_with_dlx"),
	)

	// Build queue arguments
	args := make(amqp091.Table)
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

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	_, err = ch.QueueDeclare(
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
func (r *rabbitmqClient) DeclareDLX(ctx context.Context, dlxName string, options DLXOptions) error {
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

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	err = ch.ExchangeDeclare(
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
func (r *rabbitmqClient) DeclareDLQ(ctx context.Context, dlqName string, dlxName string, options DLQOptions) error {
	_, span := r.trace(ctx, "declare_dlq")
	defer span.End()

	span.SetAttributes(
		attribute.String("rabbitmq.dlq", dlqName),
		attribute.String("rabbitmq.dlx", dlxName),
		attribute.Bool("rabbitmq.durable", options.Durable),
		attribute.String("rabbitmq.operation", "declare_dlq"),
	)

	// Declare the DLQ
	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	_, err = ch.QueueDeclare(
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
	err = ch.QueueBind(
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
func (r *rabbitmqClient) SetupDLXForQueue(ctx context.Context, queueName, dlxName, dlqName string, options DLXOptions) error {
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
	dlqOptions := DLQOptions{
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
	args := make(amqp091.Table)
	args["x-dead-letter-exchange"] = dlxName
	args["x-dead-letter-routing-key"] = dlqName

	opCtx, cancel := withDefaultTimeout(ctx, defaultOperationTimout)
	defer cancel()

	ch, err := r.ensureChannel(opCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("rabbitmq channel not available: %w", err)
	}

	// Check if queue exists using QueueDeclare with passive mode
	_, err = ch.QueueDeclarePassive(queueName, false, false, false, false, nil)
	queueExists := err == nil

	if !queueExists {
		// If queue doesn't exist, just declare it with DLX
		if r.logger != nil {
			r.logger.Infof("Queue does not exist, will create with DLX: queue=%s", queueName)
		}
		// Declare with default durable settings
		_, err = ch.QueueDeclare(
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
		_, err = ch.QueueDelete(queueName, false, false, false)
		if err != nil {
			if r.logger != nil {
				r.logger.Warnf("Failed to delete queue, will try to declare with DLX args anyway: queue=%s, error=%s", queueName, err.Error())
			}
		}

		// Recreate with DLX args
		// Use durable=true as default for important queues
		_, err = ch.QueueDeclare(
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

// cloneHeadersWithRetryCount returns a copy of headers with x-retry-count set and x-death removed
// so republished retries do not mix DLX metadata with the explicit counter.
func cloneHeadersWithRetryCount(headers amqp091.Table, retryCount int) amqp091.Table {
	h := maps.Clone(headers)
	if h == nil {
		h = amqp091.Table{}
	}
	delete(h, "x-death")
	h["x-retry-count"] = int32(retryCount)
	return h
}

// getXDeathCountForQueue returns RabbitMQ x-death.count for the given queue, if present.
// x-death is only updated when broker dead-letters a message (reject/requeue=false, ttl, overflow...).
func (r *rabbitmqClient) getXDeathCountForQueue(delivery amqp091.Delivery, queue string) int {
	if delivery.Headers == nil {
		return 0
	}
	xDeath, ok := delivery.Headers["x-death"]
	if !ok {
		return 0
	}
	deaths, ok := xDeath.([]interface{})
	if !ok {
		return 0
	}
	for _, d := range deaths {
		m, ok := d.(amqp091.Table)
		if !ok {
			// Sometimes headers decode as map[string]interface{}.
			if mm, ok := d.(map[string]interface{}); ok {
				if q, _ := mm["queue"].(string); q == queue {
					if c, ok := mm["count"]; ok {
						switch v := c.(type) {
						case int:
							return v
						case int32:
							return int(v)
						case int64:
							return int(v)
						case float64:
							return int(v)
						}
					}
				}
			}
			continue
		}
		if q, _ := m["queue"].(string); q != queue {
			continue
		}
		if c, ok := m["count"]; ok {
			switch v := c.(type) {
			case int:
				return v
			case int32:
				return int(v)
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

// getRetryCount extracts retry count from message headers
// Returns 0 if not found or invalid
func (r *rabbitmqClient) getRetryCount(delivery amqp091.Delivery) int {
	if delivery.Headers == nil {
		return 0
	}

	// Check for x-retry-count header (our custom header)
	if retryCount, ok := delivery.Headers["x-retry-count"]; ok {
		switch v := retryCount.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}

	// Also check x-death (only present after dead-lettering). Not updated on Nack(requeue=true).
	// Length is a rough hint; prefer x-retry-count for bounded retries in this client.
	if xDeath, ok := delivery.Headers["x-death"]; ok {
		if deaths, ok := xDeath.([]interface{}); ok {
			return len(deaths)
		}
	}

	return 0
}

// Close closes the connection
func (r *rabbitmqClient) Close() error {
	var errs []error

	r.mu.Lock()
	ch := r.channel
	conn := r.conn
	r.channel = nil
	r.conn = nil
	r.mu.Unlock()

	if ch != nil {
		if err := ch.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close channel: %w", err))
			if r.logger != nil {
				r.logger.Errorf("Failed to close RabbitMQ channel: error=%s", err.Error())
			}
		}
	}

	if conn != nil {
		if err := conn.Close(); err != nil {
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
