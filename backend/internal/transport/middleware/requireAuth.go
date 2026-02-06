package custom_middlewares

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func RequireAuth(rd *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			apiKey, err := rd.Get(c.Request().Context(), "active_key").Result()
			if err != nil {
				return c.String(http.StatusInternalServerError, "Internal Server Error")
			}
			if c.Request().Header.Get("Authorization") != "Bearer "+apiKey {
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
			return next(c)
		}
	}
}
