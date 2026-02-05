package custom_middlewares

import (
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func AdminPasscodeMiddleware(ADMIN_PASS_HASH string) echo.MiddlewareFunc {
	encodedHash := ADMIN_PASS_HASH
	mwLogger := log.With().Str("module", "admin_passcode_middleware").Logger()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			pass := strings.TrimSpace(c.Request().Header.Get("X-Admin-Passcode"))
			if pass == "" {
				mwLogger.Warn().Msg("missing admin passcode")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing admin passcode")
			}
			mwLogger.Debug().Msgf("admin passcode received")
			match, err := argon2id.ComparePasswordAndHash(pass, encodedHash)

			if err != nil || !match {
				mwLogger.Warn().Msgf("invalid admin passcode")
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid admin passcode")
			}
			mwLogger.Debug().Msgf("admin passcode matched")
			defer mwLogger.Debug().Msgf("All good  on admin passcode middleware")

			return next(c)
		}
	}
}
