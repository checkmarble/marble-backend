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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const (
	// BaselineMigrationVersion is the version of the compacted baseline migration.
	// This migration contains the consolidated schema from all migrations prior to this version.
	BaselineMigrationVersion int64 = 20241231000000

	// MinimumMigrationVersion is the minimum database schema version required to run migrations.
	// This is greater than BaselineMigrationVersion, leaving some buffer to make sure users are
	// clearly past the compaction point before they switch to a compacted version.
	// This means a user must be at least on v0.36.0 before upgrading to a version with this code.
	// Databases with a version below this (but > 0) must first upgrade to an intermediate
	// Marble version before upgrading to a version with compacted migrations.
	MinimumMigrationVersion int64 = 20250218103800
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

// The argument "migrateDownTo" controls the direction of the migration and how far to migrate down.
// NB: this controls only the Marble DB migrations. River migrations are run in the up direction if Marble migrates
// up, or skipped if Marble migrates down. There is no strong need to control river migrations down for now, as
// this is typically only run once every few versions with new river versions.
func (m *Migrater) Run(ctx context.Context, migrateDownTo *int64) error {
	connectionString := m.pgConfig.GetConnectionString()
	cfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return errors.Wrap(err, "unable to parse connection string")
	}
	if m.pgConfig.ImpersonateRole != "" {
		cfg.ConnConfig.Config.AfterConnect = func(ctx context.Context, conn *pgconn.PgConn) error {
			res := conn.Exec(ctx, "SET ROLE "+m.pgConfig.ImpersonateRole)
			_, err := res.ReadAll()
			return err
		}
	}
	// we register the config with the stdlib driver to be able to use it with the goose library. This allows
	// to use even advanced settings like AfterConnect that are not supported by the sql.Open function.
	// It works by setting the config as a global variable and then using the stdlib driver to open the connection.
	// The DSN created by the driver is only used as a lookup key for the config within sql.Open().
	registeredConfig := stdlib.RegisterConnConfig(cfg.ConnConfig)

	if err := m.openDb(ctx, registeredConfig); err != nil {
		return errors.Wrap(err, "unable to open db in Migrater")
	}

	// Now run the migrations
	if err := m.runMarbleDbMigrations(ctx, migrateDownTo); err != nil {
		return errors.Wrap(err, "runMarbleDbMigrations error")
	}

	if migrateDownTo == nil {
		pgxPool, err := m.openDbPgx(ctx, cfg)
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
	}

	return nil
}

func (m *Migrater) openDb(ctx context.Context, connectionDsn string) error {
	db, err := sql.Open("pgx", connectionDsn)
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

func (m *Migrater) openDbPgx(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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

func (m *Migrater) runMarbleDbMigrations(ctx context.Context, migrateDownTo *int64) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Migrations starting to setup marble DB")
	goose.SetBaseFS(m.dbMigrationsFileSystem)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Check the database version and handle migration compaction
	if err := m.handleMigrationCompaction(ctx); err != nil {
		return err
	}

	if migrateDownTo == nil {
		return goose.Up(m.db, "migrations")
	}
	return goose.DownTo(m.db, "migrations", *migrateDownTo)
}

// handleMigrationCompaction checks the database version and handles the migration compaction scenario.
//
// For existing databases that are past the MinimumMigrationVersion, the baseline migration
// (which contains all compacted migrations) would appear as "missing" to goose. Instead of
// using goose's allow-missing feature (which could hide real issues), we explicitly insert
// the baseline version into goose's version table, marking it as applied.
//
// This ensures:
// - Fresh databases apply the baseline migration normally
// - Existing databases have the baseline marked as applied (they already have the schema)
// - Goose runs in strict mode, catching any real missing migrations
func (m *Migrater) handleMigrationCompaction(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	// Get the highest applied migration version. We use MAX(version_id) directly instead of
	// goose.GetDBVersion() because the latter returns the most recently inserted row,
	// which could be the baseline after we insert it.
	maxVersion, err := m.getMaxAppliedVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current database schema version")
	}

	logger.InfoContext(ctx, "Current database schema version",
		"version", maxVersion,
		"minimum_required", MinimumMigrationVersion)

	// Fresh database (no migrations applied) - will apply baseline migration normally
	if maxVersion == 0 {
		return nil
	}

	// Check if the baseline migration is already recorded in goose's version table.
	// If it is, we're good - either this is a new DB that applied the baseline,
	// or an old DB where we already inserted the baseline record.
	baselineExists, err := m.baselineMigrationExists(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check if baseline migration exists")
	}
	if baselineExists {
		logger.InfoContext(ctx, "Baseline migration already recorded", "version", BaselineMigrationVersion)
		return nil
	}

	// Baseline doesn't exist - this is an old database that's upgrading.
	// Check if it's at a valid version to receive the compacted migrations.
	if maxVersion < MinimumMigrationVersion {
		return errors.Newf(
			"database schema version %d is below the minimum required version %d. "+
				"Migrations prior to %d have been compacted into a baseline migration. "+
				"Please first upgrade to a Marble version released before the migration compaction, "+
				"then upgrade to this version.",
			maxVersion, MinimumMigrationVersion, MinimumMigrationVersion,
		)
	}

	// Database is at or above minimum version - mark the baseline as applied
	if err := m.insertBaselineMigration(ctx); err != nil {
		return errors.Wrap(err, "failed to mark baseline migration as applied")
	}

	logger.InfoContext(ctx, "Marked baseline migration as applied", "version", BaselineMigrationVersion)
	return nil
}

// getMaxAppliedVersion returns the highest migration version that has been applied.
// Returns 0 if no migrations have been applied (fresh database) or if the goose_db_version
// table doesn't exist yet (clean install).
func (m *Migrater) getMaxAppliedVersion(ctx context.Context) (int64, error) {
	// First check if the goose_db_version table exists
	var tableExists bool
	err := m.db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'goose_db_version')",
	).Scan(&tableExists)
	if err != nil {
		return 0, errors.Wrap(err, "failed to check if goose_db_version table exists")
	}
	if !tableExists {
		// Table doesn't exist - this is a clean install
		return 0, nil
	}

	var maxVersion sql.NullInt64
	err = m.db.QueryRowContext(ctx,
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true",
	).Scan(&maxVersion)
	if err != nil {
		return 0, err
	}
	if !maxVersion.Valid {
		return 0, nil
	}
	return maxVersion.Int64, nil
}

// baselineMigrationExists checks if the baseline migration version is recorded in goose's version table.
func (m *Migrater) baselineMigrationExists(ctx context.Context) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM goose_db_version WHERE version_id = $1",
		BaselineMigrationVersion,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// insertBaselineMigration inserts the baseline migration version into goose's version table.
// This is needed for existing databases that were migrated using the individual migrations
// that have now been compacted into the baseline.
func (m *Migrater) insertBaselineMigration(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx,
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)",
		BaselineMigrationVersion,
	)
	return err
}
