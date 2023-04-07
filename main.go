package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"marble/marble-backend/api"
	"marble/marble-backend/app"
	"marble/marble-backend/pg_repository"
)

var version = "local-dev"
var appName = "marble/marble-backend"

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	////////////////////////////////////////////////////////////
	// Init
	////////////////////////////////////////////////////////////
	log.Printf("starting %s version %s", appName, version)

	// Read ENV variables for configuration

	// Port
	env, ok := os.LookupEnv("ENV")
	if !ok || env == "" {
		env = "DEV"
		log.Printf("no ENV environment variable (default to DEV)")
	}

	// API
	// Port
	port, ok := os.LookupEnv("PORT")
	if !ok || port == "" {
		log.Fatalf("set PORT environment variable")
	}

	// Postgres
	PGHostname, ok := os.LookupEnv("PG_HOSTNAME")
	if !ok || PGHostname == "" {
		log.Fatalf("set PG_HOSTNAME environment variable")
	}

	PGPort, ok := os.LookupEnv("PG_PORT")
	if !ok || PGPort == "" {
		log.Fatalf("set PG_PORT environment variable")
	}

	PGUser, ok := os.LookupEnv("PG_USER")
	if !ok || PGUser == "" {
		log.Fatalf("set PG_USER environment variable")
	}

	PGPassword, ok := os.LookupEnv("PG_PASSWORD")
	if !ok || PGPassword == "" {
		log.Fatalf("set PG_PASSWORD environment variable")
	}

	// Output config for debug before starting
	log.Printf("Port: %v", port)

	////////////////////////////////////////////////////////////
	// Setup dependencies
	////////////////////////////////////////////////////////////

	// Postgres repository
	pgRepository, _ := pg_repository.New(PGHostname, PGPort, PGUser, PGPassword, embedMigrations)

	if env == "DEV" {
		pgRepository.Seed()
	}

	app, _ := app.New(pgRepository)
	api, _ := api.New("8080", app)

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

	log.Printf("stopping %s version %s", appName, version)
}
