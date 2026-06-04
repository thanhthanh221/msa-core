package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Config holds MongoDB connection settings.
type Config struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
}

// MongoClient provides access to a MongoDB database with tracing support.
type MongoClient interface {
	Client() *mongo.Client
	Database() *mongo.Database
	Collection(name string) *mongo.Collection
	Ping(ctx context.Context) error
	Disconnect(ctx context.Context) error
}

type mongoClient struct {
	client   *mongo.Client
	database *mongo.Database
	tracer   trace.TracerProvider
}

// NewMongoClient connects to MongoDB using the given config and tracer.
func NewMongoClient(ctx context.Context, cfg Config, tracer trace.TracerProvider) (MongoClient, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongodb: URI is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mongodb: database name is required")
	}

	connectTimeout := cfg.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 10 * time.Second
	}

	ctx, span := traceConnect(ctx, tracer, cfg)
	if span != nil {
		defer span.End()
	}

	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(connectTimeout)

	if cfg.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize > 0 {
		clientOpts.SetMinPoolSize(cfg.MinPoolSize)
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		recordSpanError(span, err)
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("mongodb.database", cfg.Database),
		)
		span.SetStatus(codes.Ok, "connected")
	}

	return &mongoClient{
		client:   client,
		database: client.Database(cfg.Database),
		tracer:   tracer,
	}, nil
}

func (m *mongoClient) Client() *mongo.Client {
	return m.client
}

func (m *mongoClient) Database() *mongo.Database {
	return m.database
}

func (m *mongoClient) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

func (m *mongoClient) Ping(ctx context.Context) error {
	ctx, span := m.trace(ctx, "mongodb.ping")
	if span != nil {
		defer span.End()
	}

	err := m.client.Ping(ctx, readpref.Primary())
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (m *mongoClient) Disconnect(ctx context.Context) error {
	ctx, span := m.trace(ctx, "mongodb.disconnect")
	if span != nil {
		defer span.End()
	}

	if m.client == nil {
		return nil
	}

	err := m.client.Disconnect(ctx)
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (m *mongoClient) trace(ctx context.Context, name string) (context.Context, trace.Span) {
	tracer := m.tracer.Tracer("mongodb.client")
	return tracer.Start(ctx, name)
}

func traceConnect(ctx context.Context, tracer trace.TracerProvider, cfg Config) (context.Context, trace.Span) {
	t := tracer.Tracer("mongodb.client")
	ctx, span := t.Start(ctx, "mongodb.connect")
	span.SetAttributes(
		attribute.String("mongodb.uri", redactURI(cfg.URI)),
		attribute.String("mongodb.database", cfg.Database),
	)
	return ctx, span
}

func redactURI(uri string) string {
	// Avoid logging credentials from mongodb://user:pass@host/...
	at := -1
	for i := 0; i < len(uri); i++ {
		if uri[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return uri
	}
	schemeEnd := 0
	for i := 0; i+2 < len(uri); i++ {
		if uri[i] == ':' && uri[i+1] == '/' && uri[i+2] == '/' {
			schemeEnd = i + 3
			break
		}
	}
	return uri[:schemeEnd] + "***@" + uri[at+1:]
}

func recordSpanError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
