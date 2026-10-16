package observability

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const echoTracerName = "quiccpos/online/echo"

// EchoOTELMiddleware is a minimal OpenTelemetry server middleware for Echo v5.
// Extracts W3C trace context from inbound headers, starts a server span,
// attaches it to the request context, and sets X-Trace-Id on the response so
// the vendor (or Grafana operators) can reference the trace directly.
func EchoOTELMiddleware(serviceName string) echo.MiddlewareFunc {
	tracer := otel.Tracer(echoTracerName)
	propagator := otel.GetTextMapPropagator()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			if req.Header.Get(SelfProbeHeader) != "" {
				return next(c) // watchdog self-probe: no span
			}
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}
			spanName := req.Method + " " + route

			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(req.Method),
					semconv.URLPath(req.URL.Path),
					semconv.URLScheme(requestScheme(req)),
					attribute.String("http.route", route),
					attribute.String("server.name", serviceName),
				),
			)
			defer span.End()

			c.SetRequest(req.WithContext(ctx))

			if sc := span.SpanContext(); sc.IsValid() {
				c.Response().Header().Set("X-Trace-Id", sc.TraceID().String())
			}

			err := next(c)

			_, status := echo.ResolveResponseStatus(c.Response(), err)
			span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(status))
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, http.StatusText(status))
			} else if status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			return err
		}
	}
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
