package custom_middlewares

import (
	"net/http"
	"quicc/online/keys"

	"github.com/labstack/echo/v4"
)

func RequireAuth(keyRing *keys.KeyRing) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") != "Bearer "+keyRing.Active.Key {
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
			return next(c)
		}
	}
}
