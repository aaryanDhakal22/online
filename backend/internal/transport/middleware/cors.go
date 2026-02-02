package custom_middlewares

import (
	"github.com/labstack/echo/v4"
)

func CORSMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set(echo.HeaderAccessControlAllowOrigin, "*")
			c.Response().Header().Set(echo.HeaderAccessControlAllowMethods, echo.GET+", "+echo.HEAD+", "+echo.PUT+", "+echo.PATCH+", "+echo.POST+", "+echo.DELETE)
			return next(c)
		}
	}
}
