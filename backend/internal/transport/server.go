package transport

import (
	"context"
	"fmt"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

func StartServer(ctx context.Context, e *echo.Echo, port string) {
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func AddDefaultMiddlewares(e *echo.Echo, logger zerolog.Logger, domain string) {
	l := logger.With().Str("module", "middleware").Logger()
	// Add default middlewares
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())

	// Add CORS middleware conditionally
	if domain != "localhost" {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{
				fmt.Sprintf("https://%v", domain),
				fmt.Sprintf("http://%v", domain),
			},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}))
	} else {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}))
	}

	// Sane default
	e.Use(middleware.RequestID())

	e.Use(middleware.BodyLimit("5M"))

	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(10)))

	// Implement a custom logger

	e.Use(custom_middlewares.NewSimpleReqLogger(l))
}
