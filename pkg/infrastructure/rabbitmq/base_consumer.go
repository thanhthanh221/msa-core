package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/thanhthanh221/msa-core/pkg/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ActionHandler func(ctx context.Context, message *RabbitMQMessageBase) *common.ErrorResponse

// ConsumerHandler provides entity-specific logic via an action->handler map.
// Consumers only register the actions they support, avoiding "fat" interfaces.
type ConsumerHandler interface {
	ConsumerName() string
	Handlers() map[string]ActionHandler
}

type BaseConsumer struct {
	client    RabbitMQClient
	logger    *logrus.Logger
	exchange  string
	queueName string
	handler   ConsumerHandler
	tracer    trace.Tracer
}

func NewBaseConsumer(
	client RabbitMQClient,
	logger *logrus.Logger,
	exchange string,
	queueName string,
	handler ConsumerHandler,
) *BaseConsumer {
	return NewBaseConsumerWithTracer(client, logger, nil, exchange, queueName, handler)
}

func NewBaseConsumerWithTracer(
	client RabbitMQClient,
	logger *logrus.Logger,
	tracerProvider trace.TracerProvider,
	exchange string,
	queueName string,
	handler ConsumerHandler,
) *BaseConsumer {
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}
	return &BaseConsumer{
		client:    client,
		logger:    logger,
		exchange:  exchange,
		queueName: queueName,
		handler:   handler,
		tracer:    tracerProvider.Tracer("msa.consumers"),
	}
}

func (b *BaseConsumer) Start(ctx context.Context) *common.ErrorResponse {
	b.logger.Infof("%s: using queue %s bound to exchange %s", b.handler.ConsumerName(), b.queueName, b.exchange)

	consumeOptions := ConsumeOptions{
		AutoAck: false,
	}

	handler := MessageHandler(func(ctx context.Context, delivery amqp091.Delivery) error {
		if errResp := b.handleMessage(ctx, delivery); errResp != nil {
			return fmt.Errorf("error code: %d, message: %s", errResp.Code, errResp.Message)
		}
		return nil
	})

	if err := b.client.ConsumeWithOptions(ctx, b.queueName, handler, consumeOptions); err != nil {
		b.logger.Errorf("%s: failed to start consuming from queue %s: %v", b.handler.ConsumerName(), b.queueName, err)
		return common.CreateErrorResponse(common.INTERNAL_ERROR, common.TWithContext(ctx, common.MsgErrorInternal))
	}

	b.logger.Infof("%s: started consuming messages from queue %s", b.handler.ConsumerName(), b.queueName)
	return nil
}

func (b *BaseConsumer) handleMessage(ctx context.Context, delivery amqp091.Delivery) *common.ErrorResponse {
	ctx, span := b.tracer.Start(
		ctx,
		fmt.Sprintf("consumer.%s.handle", b.handler.ConsumerName()),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination", b.queueName),
			attribute.String("messaging.rabbitmq.routing_key", delivery.RoutingKey),
			attribute.String("messaging.message_id", delivery.MessageId),
			attribute.String("rabbitmq.exchange", b.exchange),
		),
	)
	defer span.End()

	b.logger.Infof("%s: received message - RoutingKey: %s, MessageID: %s",
		b.handler.ConsumerName(), delivery.RoutingKey, delivery.MessageId)

	var message RabbitMQMessageBase
	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		b.logger.Errorf("%s: failed to unmarshal message: %v", b.handler.ConsumerName(), err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return common.CreateErrorResponse(common.INTERNAL_ERROR, common.TWithContext(ctx, common.MsgErrorInternal))
	}

	span.SetAttributes(
		attribute.String("rabbitmq.action", message.Action),
		attribute.String("rabbitmq.entity_type", message.EntityType),
		attribute.String("rabbitmq.entity_id", message.EntityID),
		attribute.String("rabbitmq.message_id", message.MessageID),
	)

	if errResp := b.dispatch(ctx, &message); errResp != nil {
		b.logger.Errorf("%s: failed to process message %s: code=%d, message=%s",
			b.handler.ConsumerName(), message.MessageID, errResp.Code, errResp.Message)
		span.SetStatus(codes.Error, errResp.Message)
		return errResp
	}

	b.logger.Infof("%s: successfully processed message %s (Action: %s, EntityID: %s)",
		b.handler.ConsumerName(), message.MessageID, message.Action, message.EntityID)
	return nil
}

func (b *BaseConsumer) dispatch(ctx context.Context, message *RabbitMQMessageBase) *common.ErrorResponse {
	handlers := b.handler.Handlers()
	if handlers == nil {
		handlers = map[string]ActionHandler{}
	}

	if h, ok := handlers[message.Action]; ok && h != nil {
		return h(ctx, message)
	}

	b.logger.Warnf("%s: unsupported action %s for message %s", b.handler.ConsumerName(), message.Action, message.MessageID)
	return common.CreateErrorResponse(common.INTERNAL_ERROR, common.TWithContext(ctx, common.MsgErrorInternal))
}

// DecodeData decodes message.Data (an "any") into a typed struct.
// It centralizes the marshal/unmarshal boilerplate and keeps entity handlers small.
func DecodeData[T any](data any) (T, error) {
	var out T
	raw, err := json.Marshal(data)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}
