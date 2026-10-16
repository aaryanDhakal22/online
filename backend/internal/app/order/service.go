package orderApp

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"quicc/online/internal/observability"
)

const tracerName = "quiccpos/online/order"

// EventPublisher is the seam between this service and the SQS broker. The
// body is forwarded verbatim — no marshalling, no inspection.
type EventPublisher interface {
	Publish(ctx context.Context, orderID string, body []byte) error
}

type OrderService struct {
	mb     EventPublisher
	logger zerolog.Logger
	meters observability.Meters
	tracer trace.Tracer
}

func NewOrderService(mb EventPublisher, logger zerolog.Logger, meters observability.Meters) *OrderService {
	return &OrderService{
		mb:     mb,
		logger: logger.With().Str("service", "order").Logger(),
		meters: meters,
		tracer: otel.Tracer(tracerName),
	}
}

// Relay forwards the opaque body to SQS and records the outcome via metrics
// + a child span (so Tempo shows handler → relay → SQS as a chain).
func (s *OrderService) Relay(ctx context.Context, cmd RelayOrderCommand) (*RelayOrderResult, error) {
	ctx, span := s.tracer.Start(ctx, "order.relay",
		trace.WithAttributes(
			attribute.String("order.id", cmd.OrderID),
			attribute.Int("order.body.bytes", len(cmd.Body)),
		),
	)
	defer span.End()

	start := time.Now()

	s.meters.OrdersReceived.Add(ctx, 1)
	s.meters.BodyBytes.Record(ctx, int64(len(cmd.Body)))

	s.logger.Debug().Ctx(ctx).Str("order_id", cmd.OrderID).Int("body_bytes", len(cmd.Body)).
		Msg("relay: publish starting")

	if err := s.mb.Publish(ctx, cmd.OrderID, cmd.Body); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		s.meters.OrdersRejected.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "sqs_error")))
		s.logger.Error().Err(err).Ctx(ctx).Str("order_id", cmd.OrderID).Msg("relay: publish failed")
		return nil, err
	}

	latency := float64(time.Since(start).Milliseconds())
	s.meters.OrdersRelayed.Add(ctx, 1)
	s.meters.OrderRelayLatency.Record(ctx, latency)
	span.SetAttributes(attribute.Float64("order.relay.latency_ms", latency))

	s.logger.Info().Ctx(ctx).Str("order_id", cmd.OrderID).Int("body_bytes", len(cmd.Body)).
		Float64("latency_ms", latency).Msg("relay: order relayed")

	return &RelayOrderResult{
		Status:     "relayed",
		ExtOrderID: fmt.Sprintf("brygid-%s", cmd.OrderID),
		BodyBytes:  len(cmd.Body),
	}, nil
}
