package repositories

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// embed migrations sql folder
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

type Migrater struct {
	dbMigrationsFileSystem embed.FS
	pgConfig               infra.PgConfig
	db                     *sql.DB
}

func NewMigrater(pgConfig infra.PgConfig) *Migrater {
	return &Migrater{
		dbMigrationsFileSystem: embedMigrations,
		pgConfig:               pgConfig,
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

	pgxPool, err := m.openDbPgx(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to open db in Migrater")
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pgxPool), nil)
	if err != nil {
		return errors.Wrap(err, "unable to create migrator")
	}

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return errors.Wrap(err, "unable to run migrations")
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

func (m *Migrater) openDbPgx(ctx context.Context) (*pgxpool.Pool, error) {
	connectionString := m.pgConfig.GetConnectionString()
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
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
