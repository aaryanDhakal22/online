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

	v1.GET("/verify", handler.Verify)

	v1.GET("/generate", handler.Generate)

	// Admin middleware
	v1.GET("/set", handler.Set, cms.AdminMiddleware)

	// Protected middleware
	v1.POST("/order", handler.CreateOrder, cms.AuthMiddleware)

	v1.GET("/order/todays", handler.GetTodaysOrders, cms.AuthMiddleware)

	v1.GET("/order/latest", handler.GetLatestOrder, cms.AuthMiddleware)
}
