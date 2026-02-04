package main

import (
	"context"
	"database/sql"
	"fmt"
	"quicc/online/internal/infra/config"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/shared"
	"quicc/online/internal/transport"

	keyApp "quicc/online/internal/app/key"

	handler "quicc/online/internal/transport/handlers"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	// Import sqlite3 driver
	_ "modernc.org/sqlite"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file, please check your .env file")
	}

	// Config Setup
	cfg := config.NewConfig()

	// Logger Setup
	logger := shared.NewLogger(cfg.LogLevel, cfg.LogOutput, cfg.LogStyle)

	// Database Setup
	db, err := sql.Open("sqlite", "/tmp/app.db")
	if err != nil {
		logger.Fatal().Err(err).Msg("Error opening database")
		return
	}
	defer db.Close()

	logger.Info().Msg("Connected to database")
	// Setup Redis
	logger.Info().Msg("Connecting to Redis")
	logger.Info().Msg(fmt.Sprintf("Connecting to Redis on %s", cfg.RedisPort))

	logger.Info().Msg(fmt.Sprintf("Connecting to Redis with password %s", cfg.RedisPassword))
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("redis:%s", cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})
	// Test Redis
	logger.Info().Msg("Testing Redis connection")
	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error connecting to Redis")
		return
	}
	logger.Info().Msg("Connected to Redis")

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

	transport.StartServer(context.Background(), server, cfg.ServerPort)
}
