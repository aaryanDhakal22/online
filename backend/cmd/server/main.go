package main

import (
	"database/sql"
	"fmt"
	"net/http"

	keyApp "quicc/online/internal/app/key"
	"quicc/online/internal/infra/config"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/shared"
	"quicc/online/internal/transport"
	handler "quicc/online/internal/transport/handlers"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	// Config Setup
	cfg := config.NewConfig()

	// Logger Setup
	logger := shared.NewLogger(cfg.LogLevel, cfg.LogOutput, cfg.LogStyle)

	// Database Setup
	db, err := sql.Open("sqlite3", "/tmp/app.db")
	if err != nil {
		logger.Fatal().Err(err).Msg("Error opening database")
		return
	}
	defer db.Close()

	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	// Setup KeyStore
	keyQueries := models.New(db)
	keyStore := repositories.NewKeyRepository(keyQueries, logger)
	keyService := keyApp.NewKeyService(keyStore, redisClient, logger)

	// Setup Middlewares
	authMiddleware := custom_middlewares.RequireAuth(redisClient)
	adminMiddleware := custom_middlewares.AdminPasscodeMiddleware(cfg.AdminPassHash)

	// Setup Handlers
	handler := handler.NewHandler(keyService, logger)

	// Setup Server
	server := echo.New()

	transport.RegisterRoutes(server, &transport.CMS{
		AuthMiddleware:  authMiddleware,
		AdminMiddleware: adminMiddleware,
	}, handler)

	api.GET("/v1/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	protected.POST("/v1/orders", func(c echo.Context) error {
		newOrder := new(Order)
		if err := c.Bind(newOrder); err != nil {
			fmt.Printf("Error: %v\n", err)
			return c.String(http.StatusBadRequest, err.Error())
		}
		fmt.Printf("New order was created: %v\n", newOrder)
		return c.JSON(http.StatusCreated, newOrder)
	})

	api.GET("/v1/generate", func(c echo.Context) error {
		err := keyStore.Generate()
		if err != nil {
			return c.String(http.StatusInternalServerError, "Unable to generate key")
		}
		primedKey := keyStore.Primed
		return c.JSON(http.StatusOK, primedKey)
	})

	admin.GET("/v1/set", func(c echo.Context) error {
		err := keyStore.Use()
		if err != nil {
			return c.String(http.StatusInternalServerError, "Unable to use key")
		}
		activeKey := keyStore.Active
		activeKey.Status = "Active"
		return c.JSON(http.StatusOK, activeKey)
	})

	protected.GET("/v1/verify", func(c echo.Context) error {
		return c.String(http.StatusOK, "Verified")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
