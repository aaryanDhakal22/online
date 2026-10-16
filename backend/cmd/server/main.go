package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quicc/online/internal/infra/config"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/infra/message"
	"quicc/online/internal/migrations"
	"quicc/online/internal/observability"
	"quicc/online/internal/shared"
	"quicc/online/internal/transport"

	keyApp "quicc/online/internal/app/key"
	orderApp "quicc/online/internal/app/order"

	handler "quicc/online/internal/transport/handlers"
	custom_middlewares "quicc/online/internal/transport/middleware"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	_ "modernc.org/sqlite"
)

// version + commit are injected at build time via -ldflags. CI passes them
// from VERSION + the git sha so the running binary can be inspected via
// the /api/v1/version endpoint.
var (
	version = "dev"
	commit  = "unknown"
)

const bootstrapTracerName = "quiccpos/online/bootstrap"

// applySchema runs the embedded migration script in its own root span so
// startup hiccups show up in Tempo as well as in the logs.
func applySchema(ctx context.Context, db *sql.DB, logger zerolog.Logger) {
	tracer := otel.Tracer(bootstrapTracerName)
	ctx, span := tracer.Start(ctx, "schema.apply", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	logger.Debug().Ctx(ctx).Msg("schema: reading 001_setup.sql from embedded FS")
	schema, err := migrations.FS.ReadFile("sql/001_setup.sql")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ReadFile failed")
		logger.Fatal().Err(err).Msg("failed to read schema file")
	}
	logger.Debug().Ctx(ctx).Int("bytes", len(schema)).Msg("schema: applying via db.ExecContext")
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ExecContext failed")
		logger.Fatal().Err(err).Msg("failed to apply schema")
	}
	logger.Info().Ctx(ctx).Msg("schema: applied")
}

func main() {
	bootstrapLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	bootstrapLogger.Info().Msg("\n\n ==== Starting online ==== \n\n")

	cfg := config.New(&bootstrapLogger)
	bootstrapLogger.Debug().
		Str("env", cfg.App.Env).
		Str("port", cfg.Server.Port).
		Str("log_style", cfg.Log.Style).
		Bool("otel_endpoint_set", cfg.OTEL.Endpoint != "").
		Bool("sqs_endpoint_set", cfg.SQS.Endpoint != "").
		Msg("config loaded")

	// Single cancellable context that drives the HTTP server, watchdog, and
	// OTEL batch processors. SIGINT/SIGTERM cancels everything in lockstep.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// OTEL must come up before the structured logger is wired so the OTLP
	// log bridge picks up the global LoggerProvider.
	shutdownOTEL, err := observability.Setup(ctx, observability.Config{
		Endpoint:    cfg.OTEL.Endpoint,
		ServiceName: cfg.OTEL.ServiceName,
		AppEnv:      cfg.App.Env,
		Version:     version,
	})
	if err != nil {
		bootstrapLogger.Error().Err(err).Msg("OTEL setup failed — continuing with no telemetry")
		shutdownOTEL = func(context.Context) error { return nil }
	}
	defer func() {
		if err := shutdownOTEL(context.Background()); err != nil {
			bootstrapLogger.Warn().Err(err).Msg("OTEL shutdown reported errors")
		}
	}()
	bootstrapLogger.Debug().Bool("endpoint_set", cfg.OTEL.Endpoint != "").Msg("OTEL providers installed (no-op if endpoint empty)")

	meters, err := observability.NewMeters()
	if err != nil {
		bootstrapLogger.Fatal().Err(err).Msg("failed to create metric instruments")
	}

	rawLogger := shared.NewLogger(cfg.Log.Level, cfg.Log.Style)
	logger := observability.Wire(rawLogger, cfg.OTEL.ServiceName)
	logger.Debug().Ctx(ctx).Str("style", cfg.Log.Style).Stringer("level", cfg.Log.Level).
		Msg("logger configured")

	// Watchdog: each registered goroutine pulses on activity; if any goes
	// silent for >120s, the watchdog panics and Docker restarts us.
	watchdog := observability.NewWatchdog(120*time.Second, logger, meters)
	go watchdog.Run(ctx)
	logger.Debug().Ctx(ctx).Dur("threshold", 120*time.Second).Msg("watchdog: started")

	// SQLite open + migrations
	logger.Debug().Ctx(ctx).Str("path", cfg.DBPath).Msg("db: opening sqlite")
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		logger.Fatal().Err(err).Str("db_path", cfg.DBPath).Msg("open db")
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "SELECT 1"); err != nil {
		logger.Fatal().Err(err).Msg("db ping")
	}
	logger.Debug().Ctx(ctx).Msg("db: ping ok")
	applySchema(ctx, db, logger)
	logger.Info().Ctx(ctx).Msg("db connected and migrated")

	// Wiring
	queries := models.New(db)
	keyRepo := repositories.NewKeyRepository(queries, logger)
	keySvc := keyApp.NewKeyService(keyRepo, logger, meters)
	if err := keySvc.Bootstrap(ctx, cfg.Auth.DefaultAPIKey); err != nil {
		logger.Fatal().Err(err).Msg("key bootstrap failed")
	}

	logger.Debug().Ctx(ctx).
		Str("queue_url", cfg.SQS.QueueURL).
		Str("region", cfg.SQS.Region).
		Bool("custom_endpoint", cfg.SQS.Endpoint != "").
		Msg("sqs: building broker")
	mb, err := message.NewMessageBroker(ctx, cfg.SQS.Region, cfg.SQS.QueueURL, cfg.SQS.Endpoint, cfg.Scope.TenantID, cfg.Scope.LocationID, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("sqs broker init failed")
	}
	orderSvc := orderApp.NewOrderService(mb, logger, meters)
	h := handler.NewHandler(keySvc, orderSvc, meters, logger)

	authMW := custom_middlewares.RequireAuth(keySvc, meters)
	adminMW := custom_middlewares.AdminPasscodeMiddleware(cfg.Auth.AdminPassHash, meters)

	e := echo.New()
	httpPulse := watchdog.Register("http_server")
	transport.AddDefaultMiddlewares(e, cfg.OTEL.ServiceName, cfg.App.Domain, cfg.Server.RateLimitPerSec, logger, httpPulse)
	transport.RegisterRoutes(e, &transport.CMS{
		AuthMiddleware:  authMW,
		AdminMiddleware: adminMW,
		Version:         version,
		Commit:          commit,
	}, h)

	// Self-probe: GET our own /health every 45s through the full middleware
	// chain so an idle (no vendor traffic) but healthy server keeps pulsing
	// "http_server" and the 120s watchdog doesn't restart it. A wedged server
	// can't answer its own probe, so the watchdog still catches real hangs.
	go observability.RunSelfProbe(ctx, "http://127.0.0.1:"+cfg.Server.Port+"/health", 45*time.Second, logger)

	logger.Info().Ctx(ctx).Str("port", cfg.Server.Port).Msg("starting server")
	transport.StartServer(ctx, e, cfg.Server.Port, logger)
}
