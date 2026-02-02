package transport

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func StartServer(ctx context.Context, e *echo.Echo, port string) {
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func AddDefaultMiddlewares(e *echo.Echo) {
	// Add default middlewares
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))
}
