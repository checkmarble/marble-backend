package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"marble/marble-backend/api"
	"marble/marble-backend/app"
	"marble/marble-backend/pg_repository"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("Fatal Error in connect_unix.go: %s environment variable not set.\n", k)
	}
	return v
}

func getEnvWithDefault(k string, def string) string {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		log.Printf("no %s environment variable (default to %s)\n", k, def)
		return def
	}
	return v
}

func main() {
	////////////////////////////////////////////////////////////
	// Init
	////////////////////////////////////////////////////////////

	// Read ENV variables for configuration
	env := getEnvWithDefault("ENV", "DEV")
	pgPort := getEnvWithDefault("PG_PORT", "5432")

	var (
		port       = mustGetenv("PORT")
		pgHostname = mustGetenv("PG_HOSTNAME")
		pgUser     = mustGetenv("PG_USER")
		pgPassword = mustGetenv("PG_PASSWORD")
	)

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////
	var logger *slog.Logger
	if env == "DEV" {
		textHandler := slog.NewTextHandler(os.Stderr)
		logger = slog.New(textHandler)
		jsonHandler := slog.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	} else {
		jsonHandler := slog.NewJSONHandler(os.Stderr)
		logger = slog.New(jsonHandler)
	}

	// Postgres repository
	pgRepository, _ := pg_repository.New(env, pg_repository.PGCOnfig{
		Hostname:    pgHostname,
		Port:        pgPort,
		User:        pgUser,
		Password:    pgPassword,
		MigrationFS: embedMigrations,
	}, logger)

	if env == "DEV" {
		pgRepository.Seed()
	}

	app, _ := app.New(pgRepository)
	api, _ := api.New(port, app, logger)

	////////////////////////////////////////////////////////////
	// Start serving the app
	////////////////////////////////////////////////////////////

	// Intercept SIGxxx signals
	notify, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting server on port %v\n", port)
		if err := api.ListenAndServe(); err != nil {
			log.Println(fmt.Errorf("error serving the app: %w", err))
		}
		log.Println("server returned")
	}()

	// Block until we receive our signal.
	<-notify.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	api.Shutdown(shutdownCtx)

}
