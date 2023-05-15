package main

import (
	"context"
	"flag"
	"log"
	"marble/marble-backend/api"
	"marble/marble-backend/app"
	"marble/marble-backend/pg_repository"
	"marble/marble-backend/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

func runServer(pgRepository *pg_repository.PGRepository, port string, env string, logger *slog.Logger) {
	ctx := context.Background()

	devEnv := env == "DEV"

	corsAllowLocalhost := devEnv

	if devEnv || env == "staging" {
		pgRepository.Seed()
	}

	app, _ := app.New(pgRepository)
	api, _ := api.New(port, app, logger, api.NewSigningSecrets(), corsAllowLocalhost)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			logger.ErrorCtx(ctx, "error serving the app: \n"+err.Error())
		}
		logger.InfoCtx(ctx, "server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)
}

func main() {
	var (
		env        = utils.GetStringEnv("ENV", "DEV")
		port       = utils.GetRequiredStringEnv("PORT")
		pgPort     = utils.GetStringEnv("PG_PORT", "5432")
		pgHostname = utils.GetRequiredStringEnv("PG_HOSTNAME")
		pgUser     = utils.GetRequiredStringEnv("PG_USER")
		pgPassword = utils.GetRequiredStringEnv("PG_PASSWORD")
	)

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	var logger *slog.Logger
	if env == "DEV" {
		textHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewTextHandler(os.Stderr)
		logger = slog.New(textHandler)
	} else {
		jsonHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	}

	pgConfig := pg_repository.PGConfig{
		Hostname: pgHostname,
		Port:     pgPort,
		User:     pgUser,
		Password: pgPassword,
	}

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldWipeDb := flag.Bool("wipe", false, "Truncate db tables")
	flag.Parse()
	logger.DebugCtx(context.Background(), "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldWipeDb {
		pg_repository.WipeDb(env, pgConfig, logger)
	}
	if *shouldRunMigrations {
		pg_repository.RunMigrations(env, pgConfig, logger)
	}
	if *shouldRunServer {
		pgRepository, err := pg_repository.New(env, pgConfig)
		if err != nil {
			logger.Error("error creating pg repository:\n", err.Error())
		}
		runServer(pgRepository, port, env, logger)
	}
}
