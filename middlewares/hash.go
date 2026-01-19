package custom_middlewares

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/labstack/echo/v4"
)

func AdminPasscodeMiddleware() echo.MiddlewareFunc {
	encodedHash := strings.TrimSpace(os.Getenv("ADMIN_PASS_HASH"))
	fmt.Println("The encoded hash is: ", encodedHash)
	if encodedHash == "" {
		panic("ADMIN_PASS_HASH not set")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			pass := extractBearer(c.Request().Header.Get("Authorization"))
			if pass == "" {
				pass = strings.TrimSpace(c.Request().Header.Get("X-Admin-Passcode"))
			}
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

func extractBearer(h string) string {
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}
