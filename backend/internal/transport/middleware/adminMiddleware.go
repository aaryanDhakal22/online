package custom_middlewares

import (
	"fmt"
	"net/http"
	"quicc/online/internal/observability"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AdminPasscodeMiddleware validates the X-Admin-Passcode header against an
// argon2id hash from config. Three failure paths each emit a distinct log
// line + metric counter so operators can tell "wrong passcode" from
// "broken Doppler config" without guessing. We log only the header
// length, never its value.
func AdminPasscodeMiddleware(passHash string, meters observability.Meters) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()
			log := zerolog.Ctx(ctx)
			log.Debug().Msg("admin auth: checking")
			fmt.Println("admin auth: checking")

			pass := strings.TrimSpace(c.Request().Header.Get("X-Admin-Passcode"))
			if pass == "" {
				meters.AuthAdminDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "missing")))
				log.Warn().Ctx(ctx).Msg("admin auth: X-Admin-Passcode header missing")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing admin passcode")
			}
			log.Debug().Ctx(ctx).Int("pass_len", len(pass)).Msg("admin auth: header received, comparing")

			ok, err := argon2id.ComparePasswordAndHash(pass, passHash)
			if err != nil {
				meters.AuthAdminDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "misconfigured")))
				log.Error().Err(err).Ctx(ctx).
					Int("pass_len", len(pass)).
					Msg("admin auth: ADMIN_PASS_HASH could not be parsed — check Doppler config")
				return echo.NewHTTPError(http.StatusUnauthorized, "admin auth misconfigured")
			}
			if !ok {
				meters.AuthAdminDenied.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "mismatch")))
				log.Warn().Ctx(ctx).
					Int("pass_len", len(pass)).
					Msg("admin auth: passcode did not match ADMIN_PASS_HASH")
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid admin passcode")
			}

			meters.AuthAdminAllowed.Add(ctx, 1)
			log.Debug().Ctx(ctx).Msg("admin auth: granted")
			return next(c)
		}
	}
}
