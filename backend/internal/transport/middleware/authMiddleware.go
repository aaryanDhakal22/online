package custom_middlewares

import (
	"net/http"

	keyApp "quicc/online/internal/app/key"
	"quicc/online/internal/observability"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RequireAuth enforces a Bearer token check against the in-process key
// cache (backed by SQLite). Each branch increments AuthBearerAllowed or
// AuthBearerDenied (with a `reason` attribute) so the dashboard can show
// auth-failure rates by cause; debug logs trace the success path so
// operators can confirm a known-good token is taking the right route.
func RequireAuth(keySvc *keyApp.KeyService, meters observability.Meters) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()
			log := zerolog.Ctx(ctx)

			authHdr := c.Request().Header.Get(echo.HeaderAuthorization)
			const prefix = "Bearer "
			if authHdr == "" {
				meters.AuthBearerDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "missing")))
				log.Warn().Ctx(ctx).Msg("auth: Authorization header missing")
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
			if len(authHdr) <= len(prefix) || authHdr[:len(prefix)] != prefix {
				meters.AuthBearerDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "malformed")))
				log.Warn().Ctx(ctx).Int("hdr_len", len(authHdr)).Msg("auth: Authorization header malformed")
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}

			token := authHdr[len(prefix):]
			log.Debug().Ctx(ctx).Int("token_len", len(token)).Msg("auth: bearer token received")

			ok, err := keySvc.Verify(ctx, token)
			if err != nil {
				meters.AuthBearerDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "verify_error")))
				log.Error().Err(err).Ctx(ctx).Msg("auth: Verify returned error")
				return c.String(http.StatusInternalServerError, "Internal Server Error")
			}
			if !ok {
				meters.AuthBearerDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "mismatch")))
				log.Warn().Ctx(ctx).Int("token_len", len(token)).Msg("auth: token did not match active key")
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}

			meters.AuthBearerAllowed.Add(ctx, 1)
			log.Debug().Ctx(ctx).Msg("auth: bearer accepted")
			return next(c)
		}
	}
}
