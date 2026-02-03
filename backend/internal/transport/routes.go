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

	protected := api.Group("")

	protected.Use(cms.AuthMiddleware)

	admin := api.Group("")

	admin.Use(cms.AdminMiddleware)

	// protected.POST("/v1/orders", func(c echo.Context) error {
	// 	newOrder := new(Order)
	// 	if err := c.Bind(newOrder); err != nil {
	// 		fmt.Printf("Error: %v\n", err)
	// 		return c.String(http.StatusBadRequest, err.Error())
	// 	}
	// 	fmt.Printf("New order was created: %v\n", newOrder)
	// 	return c.JSON(http.StatusCreated, newOrder)
	// })
	//

	v1.GET("/generate", handler.Generate)

	admin.GET("/set", handler.Set)

	protected.GET("/v1/verify", handler.Verify)
}
