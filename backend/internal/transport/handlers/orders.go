package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	orderApp "quicc/online/internal/app/order"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// CreateOrder reads the inbound POST body verbatim, hashes it for an order
// ID + dedup key, attaches the digest and size to the active span, and
// hands the bytes to the order service for relay. It never unmarshalls the
// body, never inspects fields, never logs the body. PII stays opaque.
func (h *Handler) CreateOrder(c *echo.Context) error {
	ctx := c.Request().Context()
	span := trace.SpanFromContext(ctx)

	h.log.Debug().Ctx(ctx).Msg("order: reading request body")
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		span.RecordError(err)
		h.meters.OrdersRejected.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "read_error")))
		h.log.Warn().Err(err).Ctx(ctx).Msg("order: failed to read body")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "could not read request body"})
	}
	if len(body) == 0 {
		h.meters.OrdersRejected.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "empty_body")))
		h.log.Warn().Ctx(ctx).Msg("order: empty body rejected")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "empty request body"})
	}

	sum := sha256.Sum256(body)
	orderID := hex.EncodeToString(sum[:])

	span.SetAttributes(
		attribute.String("order.id", orderID),
		attribute.Int("order.body.bytes", len(body)),
	)
	h.log.Debug().Ctx(ctx).Str("order_id", orderID).Int("body_bytes", len(body)).
		Msg("order: body hashed, handing off to relay")

	res, err := h.orderSvc.Relay(ctx, orderApp.RelayOrderCommand{
		OrderID: orderID,
		Body:    body,
	})
	if err != nil {
		// Service has already incremented OrdersRejected{reason=sqs_error}
		// and recorded the error on the span via its own logging.
		h.log.Warn().Ctx(ctx).Str("order_id", orderID).Msg("order: relay failed, returning 502")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error":   "relay failed",
			"message": "the order was received but could not be forwarded to SQS",
		})
	}

	h.log.Debug().Ctx(ctx).Str("order_id", orderID).Msg("order: relay succeeded, returning 202")
	return c.JSON(http.StatusAccepted, res)
}
