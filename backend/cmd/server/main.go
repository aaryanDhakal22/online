package main

import (
	"context"
	"database/sql"
	"fmt"

	"quicc/online/internal/infra/config"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/infra/message"
	"quicc/online/internal/infra/notify"
	"quicc/online/internal/migrations"
	"quicc/online/internal/shared"
	"quicc/online/internal/transport"

	keyApp "quicc/online/internal/app/key"
	orderApp "quicc/online/internal/app/order"

	handler "quicc/online/internal/transport/handlers"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	// Import sqlite3 driver
	_ "modernc.org/sqlite"
)

func applySchema(db *sql.DB) {
	// Open the schema file
	schemaFile, err := migrations.FS.ReadFile("sql/001_setup.sql")
	if err != nil {
		fmt.Println("Error opening schema file")
		panic(err)
	}
	log.Debug().Msg("Schema file opened")
	if _, err := db.Exec(string(schemaFile)); err != nil {
		fmt.Println("Error applying schema")
		panic(err)
	}
	log.Debug().Msg("Schema applied")
}

func main() {
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

	applySchema(db)

	defer db.Close()

	logger.Info().Msg("Connected to database")

	// Setup Redis
	logger.Debug().Msg("Connecting to Redis")
	logger.Debug().Msg(fmt.Sprintf("Connecting to Redis on %s", cfg.RedisPort))

	logger.Debug().Msg(fmt.Sprintf("Connecting to Redis with password %s", cfg.RedisPassword))
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("redis:%s", cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})
	// Test Redis
	logger.Debug().Msg("Testing Redis connection")
	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error connecting to Redis")
		return
	}
	logger.Info().Msg("Connected to Redis")

	// Setup Message Broker
	logger.Debug().Msg("Connecting to Message Broker")
	mb := message.NewMessageBroker(cfg.Queuename, logger)
	logger.Info().Msg("Connected to Message Broker")

	// Setup Notifier
	notifier := notify.NewNotifier(cfg.PushoverAppToken, cfg.PushoverUsers)

	// Setup KeyStore
	keyQueries := models.New(db)
	keyStore := repositories.NewKeyRepository(keyQueries, logger)
	keyService := keyApp.NewKeyService(keyStore, redisClient, logger)

	// Setup OrderStore
	orderQueries := models.New(db)
	orderStore := repositories.NewOrderRepository(orderQueries, logger)
	orderService := orderApp.NewOrderService(orderStore, mb, logger)

	// Setup Middlewares
	authMiddleware := custom_middlewares.RequireAuth(redisClient)
	adminMiddleware := custom_middlewares.AdminPasscodeMiddleware(cfg.AdminPassHash)

	// Setup Handlers
	handler := handler.NewHandler(keyService, orderService, notifier, logger)

	// Setup Server
	server := echo.New()

	server.HideBanner = true
	server.HidePort = true

	transport.AddDefaultMiddlewares(server, logger, cfg.Domain)

	transport.RegisterRoutes(server, &transport.CMS{
		AuthMiddleware:  authMiddleware,
		AdminMiddleware: adminMiddleware,
	}, handler)

	transport.StartServer(context.Background(), server, cfg.ServerPort)
}
