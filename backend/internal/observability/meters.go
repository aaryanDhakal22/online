package observability

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const MeterName = "quiccpos/online"

// Meters bundles online's custom metric instruments. Auto instruments come
// from the Echo OTEL middleware and otelaws (SQS).
type Meters struct {
	// Order ingest pipeline
	OrdersReceived    metric.Int64Counter
	OrdersRelayed     metric.Int64Counter
	OrdersRejected    metric.Int64Counter   // attribute: reason
	OrderRelayLatency metric.Float64Histogram
	BodyBytes         metric.Int64Histogram

	// Auth
	AuthBearerAllowed metric.Int64Counter
	AuthBearerDenied  metric.Int64Counter // attribute: reason
	AuthAdminAllowed  metric.Int64Counter
	AuthAdminDenied   metric.Int64Counter // attribute: reason

	// Key lifecycle
	KeysGenerated metric.Int64Counter
	KeysActivated metric.Int64Counter // attribute: path (primed | break_glass)
	KeysBootstrap metric.Int64Counter // attribute: result (seeded | existing | skipped)

	// Liveness
	LivenessStale metric.Int64Counter // attribute: goroutine
}

func NewMeters() (Meters, error) {
	m := otel.Meter(MeterName)
	var ms Meters
	var err error

	mk := func(name, desc string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error) {
		o := append([]metric.Int64CounterOption{metric.WithDescription(desc)}, opts...)
		c, err := m.Int64Counter(name, o...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return c, nil
	}

	if ms.OrdersReceived, err = mk("orders.received", "Inbound order POSTs accepted by online/"); err != nil {
		return Meters{}, err
	}
	if ms.OrdersRelayed, err = mk("orders.relayed", "Orders successfully published to SQS"); err != nil {
		return Meters{}, err
	}
	if ms.OrdersRejected, err = mk("orders.rejected", "Orders rejected (reason attribute: read_error|empty_body|sqs_error)"); err != nil {
		return Meters{}, err
	}

	// Named without a unit suffix: the Collector's prometheusremotewrite
	// exporter appends the unit ("_milliseconds") itself, so "..._ms" would
	// double up as "..._ms_milliseconds" in Prometheus.
	if ms.OrderRelayLatency, err = m.Float64Histogram(
		"orders.relay.latency",
		metric.WithDescription("End-to-end latency from request receive to SQS ack"),
		metric.WithUnit("ms"),
	); err != nil {
		return Meters{}, fmt.Errorf("orders.relay.latency: %w", err)
	}
	if ms.BodyBytes, err = m.Int64Histogram(
		"orders.body.bytes",
		metric.WithDescription("Order body size in bytes"),
		metric.WithUnit("By"),
	); err != nil {
		return Meters{}, fmt.Errorf("orders.body.bytes: %w", err)
	}

	if ms.AuthBearerAllowed, err = mk("auth.bearer.allowed", "Bearer-token auth checks that succeeded"); err != nil {
		return Meters{}, err
	}
	if ms.AuthBearerDenied, err = mk("auth.bearer.denied", "Bearer-token auth checks that failed (reason: missing|malformed|mismatch|verify_error)"); err != nil {
		return Meters{}, err
	}
	if ms.AuthAdminAllowed, err = mk("auth.admin.allowed", "Admin passcode checks that succeeded"); err != nil {
		return Meters{}, err
	}
	if ms.AuthAdminDenied, err = mk("auth.admin.denied", "Admin passcode checks that failed (reason: missing|misconfigured|mismatch)"); err != nil {
		return Meters{}, err
	}

	if ms.KeysGenerated, err = mk("keys.generated", "API keys minted via /generate (count includes the break-glass path)"); err != nil {
		return Meters{}, err
	}
	if ms.KeysActivated, err = mk("keys.activated", "API keys promoted to active (path attribute: primed|break_glass)"); err != nil {
		return Meters{}, err
	}
	if ms.KeysBootstrap, err = mk("keys.bootstrap", "Bootstrap outcomes at startup (result attribute: seeded|existing|skipped)"); err != nil {
		return Meters{}, err
	}

	if ms.LivenessStale, err = mk("liveness.stale", "Watchdog ticks where a registered goroutine heartbeat exceeded threshold (goroutine attribute)"); err != nil {
		return Meters{}, err
	}

	return ms, nil
}
