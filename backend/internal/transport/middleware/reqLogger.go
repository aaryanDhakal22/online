package custom_middlewares

import (
	"quicc/online/internal/observability"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

// ContextLogger attaches the application logger to the request context so
// downstream middleware and handlers can pull it via zerolog.Ctx(ctx) and
// emit at the configured level. Without this middleware in front, Ctx(ctx)
// returns zerolog's Disabled logger sentinel and every .Debug()/.Info()
// call against it silently no-ops — the level isn't even checked.
//
// Install this once, AFTER observability.EchoOTELMiddleware (so the span
// is in the context when the logger gets bound) and BEFORE any middleware
// or handler that reaches for zerolog.Ctx(ctx).
func ContextLogger(l zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := l.WithContext(req.Context())
			c.SetRequest(req.WithContext(ctx))
			return next(c)
		}
	}
}

// NewSimpleReqLogger logs one structured event per request. Trace + span
// IDs are attached automatically by observability.Wire's traceHook because
// we set the request context on the logger via Ctx(ctx).
func NewSimpleReqLogger(l zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := req.Context()
			if req.Header.Get(observability.SelfProbeHeader) != "" {
				return next(c) // watchdog self-probe: don't log every interval
			}

			l.Info().Ctx(ctx).
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Str("real_ip", c.RealIP()).
				Msg("request")

			return next(c)
		}
	}
}

// PulseMiddleware refreshes a watchdog heartbeat on every request so the
// liveness watcher knows the HTTP server is still serving.
func PulseMiddleware(pulse func()) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			pulse()
			return next(c)
		}
	}
}
