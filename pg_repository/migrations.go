package pg_repository

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"marble/marble-backend/utils"

	"github.com/pressly/goose/v3"
	"golang.org/x/exp/slog"
)

// embed migrations sql folder
//
//go:embed migrations_core/*.sql
var embedMigrations embed.FS

//go:embed migrations_test_org/*.sql
var embedMigrationsTestOrg embed.FS

type migrationParams struct {
	fileSystem   embed.FS
	folderName   string
	allowMissing bool
}

func setupDbConnection(env string, pgConfig PGConfig) (*sql.DB, error) {
	connectionString := pgConfig.GetConnectionString(env)

	migrationDB, err := sql.Open("pgx", connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	err = migrationDB.Ping()
	if err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return migrationDB, nil
}

func RunMigrations(env string, pgConfig PGConfig, logger *slog.Logger) {
	db, err := setupDbConnection(env, pgConfig)
	if err != nil {
		log.Fatalln(err)
	}

	// Run core migrations, then test ingestion db migrations
	if err := runMigrationsWithFolder(db, migrationParams{fileSystem: embedMigrations, folderName: "migrations_core", allowMissing: false}, logger); err != nil {
		log.Fatalln(err)
	}
	if env == "DEV" {
		if err := runMigrationsWithFolder(db, migrationParams{fileSystem: embedMigrationsTestOrg, folderName: "migrations_test_org", allowMissing: true}, logger); err != nil {
			log.Fatalln(err)
		}
	}
}

func WipeDb(env string, pgConfig PGConfig, logger *slog.Logger) {
	gcpProjectId := utils.GetStringEnv("GOOGLE_CLOUD_PROJECT", "")
	if env != "DEV" && gcpProjectId != "tokyo-country-381508" {
		log.Fatal("WipeDb is only allowed in DEV or staging environment")
	}

	// Reset schema, then migrate it again to restore an empty db
	db, err := setupDbConnection(env, pgConfig)
	if err != nil {
		log.Fatalln(err)
	}

	// Teardown full db schema
	if env == "DEV" {
		if err := resetDbWithFolder(db, migrationParams{fileSystem: embedMigrationsTestOrg, folderName: "migrations_test_org", allowMissing: true}, logger); err != nil {
			log.Fatalln(err)
		}
	}
	if err := resetDbWithFolder(db, migrationParams{fileSystem: embedMigrations, folderName: "migrations_core", allowMissing: false}, logger); err != nil {
		log.Fatalln(err)
	}

	// Restore db schema
	if err := runMigrationsWithFolder(db, migrationParams{fileSystem: embedMigrations, folderName: "migrations_core", allowMissing: false}, logger); err != nil {
		log.Fatalln(err)
	}
	if env == "DEV" {
		if err := runMigrationsWithFolder(db, migrationParams{fileSystem: embedMigrationsTestOrg, folderName: "migrations_test_org", allowMissing: true}, logger); err != nil {
			log.Fatalln(err)
		}
	}
}

func runMigrationsWithFolder(db *sql.DB, params migrationParams, logger *slog.Logger) error {
	// start goose migrations
	logger.Info("Migrations starting to setup DB: " + params.folderName)
	goose.SetBaseFS(params.fileSystem)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// When running on the secondary folder containing the test org migrations, we allow missing migrations to allow out of order migrations with the main folder
	if params.allowMissing {
		if err := goose.Up(db, params.folderName, goose.WithAllowMissing()); err != nil {
			return fmt.Errorf("unable to run migrations: %w \n", err)
		}
	} else {
		if err := goose.Up(db, params.folderName); err != nil {
			return fmt.Errorf("unable to run migrations: %w \n", err)
		}
	}
	return nil
}

func resetDbWithFolder(db *sql.DB, params migrationParams, logger *slog.Logger) error {
	// start goose migrations
	logger.Info("Migrations starting to reset DB: " + params.folderName)
	goose.SetBaseFS(params.fileSystem)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// When running on the secondary folder containing the test org migrations, we allow missing migrations to allow out of order migrations with the main folder
	if params.allowMissing {
		if err := goose.Reset(db, params.folderName, goose.WithAllowMissing()); err != nil {
			return fmt.Errorf("unable to reset migrations: %w \n", err)
		}
	} else {
		if err := goose.Reset(db, params.folderName); err != nil {
			return fmt.Errorf("unable to reset migrations: %w \n", err)
		}
	}
	return nil
}
