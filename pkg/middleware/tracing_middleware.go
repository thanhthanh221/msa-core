package middleware

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates a root span for each HTTP request
// All subsequent operations will be child spans of this root span
type TracingMiddleware struct {
	tracerProvider trace.TracerProvider
	logger         *logrus.Logger
	propagator     propagation.TextMapPropagator
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(tracerProvider trace.TracerProvider, logger *logrus.Logger) *TracingMiddleware {
	return &TracingMiddleware{
		tracerProvider: tracerProvider,
		logger:         logger,
		propagator:     otel.GetTextMapPropagator(), // W3C Trace Context propagator
	}
}

// Middleware returns the echo middleware function
// Only creates trace for write operations (POST, PUT, DELETE, PATCH)
func (m *TracingMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			method := req.Method

			// Only trace write operations (POST, PUT, DELETE, PATCH)
			isWriteOperation := method == http.MethodPost ||
				method == http.MethodPut ||
				method == http.MethodDelete ||
				method == http.MethodPatch

			var ctx context.Context
			var span trace.Span

			if isWriteOperation {
				// Extract trace context from HTTP headers (for distributed tracing)
				ctx = m.propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

				// Create root span for this request
				tracer := m.tracerProvider.Tracer("http.request")
				spanName := method + " " + c.Path()
				if spanName == " " {
					spanName = method + " " + req.URL.Path
				}

				ctx, span = tracer.Start(ctx, spanName)
				defer span.End()
			} else {
				// For read operations, just use the original context
				ctx = req.Context()
			}

			if isWriteOperation {
				// Set span attributes
				span.SetAttributes(
					semconv.HTTPMethodKey.String(method),
					semconv.HTTPURLKey.String(req.URL.String()),
					semconv.HTTPRouteKey.String(c.Path()),
					attribute.String("http.user_agent", req.UserAgent()),
					semconv.HTTPRequestContentLengthKey.Int64(req.ContentLength),
				)

				// Add remote IP if available
				if ip := c.RealIP(); ip != "" {
					span.SetAttributes(attribute.String("http.client_ip", ip))
				}

				// Store span in echo context for potential use in handlers
				c.Set("span", span)
			}

			// Inject context into request context
			// This ensures all child operations use the same trace (for write ops)
			req = req.WithContext(ctx)
			c.SetRequest(req)

			// Process request
			err := next(c)

			if isWriteOperation {
				// Set response attributes
				status := c.Response().Status
				span.SetAttributes(
					semconv.HTTPStatusCodeKey.Int(status),
					semconv.HTTPResponseContentLengthKey.Int64(c.Response().Size),
				)

				// Set span status based on HTTP status code
				if status >= 400 {
					span.SetStatus(codes.Error, http.StatusText(status))
				} else {
					span.SetStatus(codes.Ok, "Request processed successfully")
				}

				// Record error if any
				if err != nil {
					span.RecordError(err)
				}
			}

			return err
		}
	}
}
