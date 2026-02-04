package transport

import (
	"net/http"

	handler "quicc/online/internal/transport/handlers"

	"github.com/labstack/echo/v4"
)

type CMS struct {
	AuthMiddleware  echo.MiddlewareFunc
	AdminMiddleware echo.MiddlewareFunc
}

func RegisterRoutes(e *echo.Echo, cms *CMS, handler *handler.Handler) {
	api := e.Group("/api")

	v1 := api.Group("/v1")

	v1.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	protected := v1.Group("")

	protected.Use(cms.AuthMiddleware)

	admin := v1.Group("")

	admin.Use(cms.AdminMiddleware)

	v1.GET("/generate", handler.Generate)

	admin.GET("/set", handler.Set)

	protected.GET("/v1/verify", handler.Verify)
}
