package repositories

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/checkmarble/marble-backend/infra"
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

type Migrater struct {
	dbMigrationsFileSystem   embed.FS
	analyticsViewsFileSystem embed.FS
	pgConfig                 infra.PgConfig
	db                       *sql.DB
}

func NewMigrater(pgConfig infra.PgConfig) *Migrater {
	return &Migrater{
		dbMigrationsFileSystem:   embedMigrations,
		analyticsViewsFileSystem: embedAnalyticsViews,
		pgConfig:                 pgConfig,
	}
}

func (m *Migrater) Run(ctx context.Context) error {
	if err := m.openDb(ctx); err != nil {
		return errors.Wrap(err, "unable to open db in Migrater")
	}

	// Now run the migrations
	if err := m.runMarbleDbMigrations(ctx); err != nil {
		return errors.Wrap(err, "runMarbleDbMigrations error")
	}

	if err := m.migrateAnalyticsViews(ctx, m.analyticsViewsFileSystem); err != nil {
		return errors.Wrap(err, "migrateAnalyticsViews error")
	}

	return nil
}

func (m *Migrater) openDb(ctx context.Context) error {
	connectionString := m.pgConfig.GetConnectionString()
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return errors.Wrap(err, "unable to create connection pool for migrations")
	} else {
		m.db = db
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = m.db.PingContext(ctx); err != nil {
		return errors.Wrap(err, "unable to ping database")
	}
	return nil
}

func (m *Migrater) runMarbleDbMigrations(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Migrations starting to setup marble DB")
	goose.SetBaseFS(m.dbMigrationsFileSystem)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(m.db, "migrations")
}

func (m *Migrater) migrateAnalyticsViews(ctx context.Context, folder embed.FS) error {
	if err := fs.WalkDir(
		folder,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				return m.createViewFromFile(ctx, folder, path)
			}

			return nil
		},
	); err != nil {
		return errors.Wrap(err, "error while walking embedded analytics_views folder")
	}
	return nil
}

func (m *Migrater) createViewFromFile(ctx context.Context, folder embed.FS, path string) error {
	logger := utils.LoggerFromContext(ctx)
	sql, err := folder.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to read file %s", path))
	}

	if _, err := m.db.Exec(string(sql)); err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to create view from file %s", path))
	}

	logger.InfoContext(ctx, fmt.Sprintf("Successfully created view from %s", path))
	return nil
}
