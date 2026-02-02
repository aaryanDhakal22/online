package custom_middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
)

func AdminPasscodeMiddleware(ADMIN_PASS_HASH string) echo.MiddlewareFunc {
	encodedHash := ADMIN_PASS_HASH

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			pass := strings.TrimSpace(c.Request().Header.Get("X-Admin-Passcode"))
			if pass == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing admin passcode")
			}
			match, err := argon2id.ComparePasswordAndHash(pass, encodedHash)
			fmt.Println("The match is: ", match)
			if err != nil || !match {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid admin passcode")
			}

			return next(c)
		}
	}
}
