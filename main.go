package main

import (
	"context"
	"embed"
	"flag"
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

func run_server(pgRepository *pg_repository.PGRepository, port string, env string) { // Read ENV variables for configuration

	if env == "DEV" {
		pgRepository.Seed()
	}

	app, _ := app.New(pgRepository)
	api, _ := api.New(port, app)

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

func run_migrations(pgConfig pg_repository.PGConfig, env string) {
	pg_repository.RunMigrations(pgConfig, env)
}

func main() {
	env := getEnvWithDefault("ENV", "DEV")
	pgPort := getEnvWithDefault("PG_PORT", "5432")

	var (
		port       = mustGetenv("PORT")
		pgHostname = mustGetenv("PG_HOSTNAME")
		pgUser     = mustGetenv("PG_USER")
		pgPassword = mustGetenv("PG_PASSWORD")
	)

	pgConfig := pg_repository.PGConfig{
		Hostname:    pgHostname,
		Port:        pgPort,
		User:        pgUser,
		Password:    pgPassword,
		MigrationFS: embedMigrations,
	}

	pgRepository, _ := pg_repository.New(env, pgConfig)

	shouldRunMigrations := flag.Bool("migrations", false, "Run migrations")
	shouldRunServer := flag.Bool("server", false, "Run server")
	flag.Parse()

	if *shouldRunMigrations {
		run_migrations(pgConfig, env)
	}
	if *shouldRunServer {
		run_server(pgRepository, port, env)
	}

}
