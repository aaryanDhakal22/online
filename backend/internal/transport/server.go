package transport

import (
	"context"
	"net/http"
	"time"

	"quicc/online/internal/observability"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
)

// StartServer runs the Echo v5 server until ctx is cancelled. The shared
// context is created in main.go via signal.NotifyContext so SIGINT/SIGTERM
// drains the server cleanly.
func StartServer(ctx context.Context, e *echo.Echo, port string, logger zerolog.Logger) {
	sc := echo.StartConfig{
		Address:         ":" + port,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadHeaderTimeout = 10 * time.Second
			return nil
		},
	}
	if err := sc.Start(ctx, e); err != nil {
		logger.Error().Err(err).Msg("server stopped")
	}
}

// AddDefaultMiddlewares installs OTEL, the context-logger binder, request
// logging, recover, and the liveness pulse — in that order so:
//  1. OTEL puts the span on the request context.
//  2. ContextLogger binds the application logger to that same context so
//     downstream code can call zerolog.Ctx(ctx).Debug()/.Info() and have
//     events actually fire (without it, Ctx returns a disabled logger).
//  3. NewSimpleReqLogger emits the per-request log line.
//
// rateLimitPerSec controls the per-IP request rate; 0 skips the limiter
// entirely (dev/test default — pytest suites fire sequential bursts that
// would otherwise tip over the bucket).
func AddDefaultMiddlewares(e *echo.Echo, serviceName, domain string, rateLimitPerSec int, logger zerolog.Logger, pulse func()) {
	e.Use(observability.EchoOTELMiddleware(serviceName))
	e.Use(custom_middlewares.ContextLogger(logger))
	e.Use(custom_middlewares.NewSimpleReqLogger(logger))
	e.Use(custom_middlewares.PulseMiddleware(pulse))
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Use(middleware.BodyLimit(5 * 1024 * 1024)) // 5 MiB
	if rateLimitPerSec > 0 {
		e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(float64(rateLimitPerSec))))
		logger.Info().Int("per_sec", rateLimitPerSec).Msg("rate limiter enabled")
	} else {
		logger.Info().Msg("rate limiter disabled (RATE_LIMIT_PER_SEC=0)")
	}
	e.Use(middleware.RequestID())

	allowedMethods := []string{
		http.MethodGet, http.MethodHead, http.MethodPut,
		http.MethodPatch, http.MethodPost, http.MethodDelete,
	}
	if domain != "localhost" {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{
				"https://" + domain,
				"http://" + domain,
			},
			AllowMethods: allowedMethods,
		}))
	} else {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: allowedMethods,
		}))
	}
}
