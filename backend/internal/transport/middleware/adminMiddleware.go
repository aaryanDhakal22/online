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
			mwLogger.Debug().Msgf("Checking if admin passcode is valid")
			reqPath := c.Request().URL.Path
			matchedPath := c.Path()
			log.Debug().Msgf("reqPath: %s, matchedPath: %s", reqPath, matchedPath)
			pass := strings.TrimSpace(c.Request().Header.Get("X-Admin-Passcode"))
			if pass == "" {
				mwLogger.Warn().Msg("Missing admin passcode header")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing admin passcode")
			}
			mwLogger.Debug().Msgf("Admin passcode received %s", pass)
			match, err := argon2id.ComparePasswordAndHash(pass, encodedHash)
			mwLogger.Debug().Msgf("Comparing admin passcode")
			if err != nil || !match {
				mwLogger.Warn().Msgf("Invalid admin passcode")
				mwLogger.Debug().Msgf("err: %s, match: %t", err, match)
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid admin passcode")
			}

			mwLogger.Debug().Msgf("admin passcode matched")
			defer mwLogger.Debug().Msgf("All good  on admin passcode middleware")

			return next(c)
		}
	}
}
