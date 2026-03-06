package custom_middlewares

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func RequireAuth(rd *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		authLogger := log.With().Str("module", "auth middleware").Logger()
		return func(c echo.Context) error {
			authLogger.Debug().Msg("Checking if user is authenticated")
			apiKey, err := rd.Get(c.Request().Context(), "active_key").Result()
			if err != nil {
				authLogger.Error().Err(err).Msg("Error getting active key")
				return c.String(http.StatusInternalServerError, "Internal Server Error")
			}
			if c.Request().Header.Get("Authorization") != "Bearer "+apiKey {
				authLogger.Error().Msg("Unauthorized")
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
			authLogger.Debug().Msg("User is authenticated")
			return next(c)
		}
	}
}
