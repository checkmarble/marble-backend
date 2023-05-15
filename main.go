package main

import (
	"context"
	"embed"
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

// embed migrations sql folder
//
//go:embed pg_repository/migrations_core/*.sql
var embedMigrations embed.FS

func runServer(pgRepository *pg_repository.PGRepository, port string, env string, logger *slog.Logger) {
	ctx := context.Background()
	if env == "DEV" {
		pgRepository.Seed()
	}

	app, _ := app.New(pgRepository)
	api, _ := api.New(port, app, logger, api.NewSigningSecrets())

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

func runMigrations(env string, pgConfig pg_repository.PGConfig, logger *slog.Logger) {
	pg_repository.RunMigrations(env, pgConfig, "pg_repository/migrations", logger, false)
}

func runWipeDb(pgConfig pg_repository.PGConfig, logger *slog.Logger) {
	env := utils.GetStringEnv("ENV", "DEV")
	gcpProjectId := utils.GetStringEnv("GOOGLE_CLOUD_PROJECT", "")
	if env != "DEV" || gcpProjectId != "tokyo-country-381508" {
		log.Fatal("WipeDb is only allowed in DEV or staging environment")
	}
	pg_repository.WipeDb(env, pgConfig, "pg_repository/migrations", logger)
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
		Hostname:    pgHostname,
		Port:        pgPort,
		User:        pgUser,
		Password:    pgPassword,
		MigrationFS: embedMigrations,
	}

	pgRepository, err := pg_repository.New(env, pgConfig)
	if err != nil {
		logger.Error("error creating pg repository:\n", err.Error())
	}

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	shouldWipeDb := flag.Bool("wipe", false, "Truncate db tables")
	flag.Parse()
	logger.DebugCtx(context.Background(), "shouldRunMigrations", *shouldRunMigrations, "shouldRunServer", *shouldRunServer)

	if *shouldWipeDb {
		runWipeDb(pgConfig, logger)
	}
	if *shouldRunMigrations {
		runMigrations(env, pgConfig, logger)
	}
	if *shouldRunServer {
		runServer(pgRepository, port, env, logger)
	}
}
