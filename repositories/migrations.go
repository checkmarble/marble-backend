package repositories

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

// embed analytics_views sql folder
//
//go:embed analytics_views/*.sql
var embedAnalyticsViews embed.FS

type migrationParams struct {
	fileSystem   embed.FS
	folderName   string
	allowMissing bool
}

func setupDbConnection(env string, pgConfig utils.PGConfig) (*sql.DB, error) {
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

func RunMigrations(env string, pgConfig utils.PGConfig, logger *slog.Logger) error {
	db, err := setupDbConnection(env, pgConfig)
	if err != nil {
		return fmt.Errorf("setupDbConnection error: %w", err)
	}

	params := migrationParams{
		fileSystem:   embedMigrations,
		folderName:   "migrations",
		allowMissing: false,
	}
	if err := runMigrationsWithFolder(db, params, logger); err != nil {
		return fmt.Errorf("runMigrationsWithFolder error: %w", err)
	}

	if err := migrateAnalyticsViews(db, embedAnalyticsViews); err != nil {
		return errors.Wrap(err, "migrateAnalyticsViews error")
	}
	return nil
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
			return fmt.Errorf("unable to run migrations: %w", err)
		}
	} else {
		if err := goose.Up(db, params.folderName); err != nil {
			return fmt.Errorf("unable to run migrations: %w", err)
		}
	}
	return nil
}

func migrateAnalyticsViews(db *sql.DB, folder embed.FS) error {
	if err := fs.WalkDir(
		folder,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				return createViewFromFile(db, folder, path)
			}

			return nil
		},
	); err != nil {
		return errors.Wrap(err, "error while walking embedded analytics_views folder")
	}
	return nil
}

func createViewFromFile(db *sql.DB, folder embed.FS, path string) error {
	s, err := folder.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to read file %s", path))
	}

	if _, err := db.Exec(string(s)); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to create view from file %s", path))
	}
	fmt.Printf("Successfully created view from %s\n", path)
	return nil
}
