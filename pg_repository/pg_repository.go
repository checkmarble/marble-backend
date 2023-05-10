package pg_repository

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"golang.org/x/exp/slog"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PgxPoolIface interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Begin(context.Context) (pgx.Tx, error)
	Close()
}

type PGConfig struct {
	Hostname         string
	Port             string
	User             string
	Password         string
	ConnectionString string
	MigrationFS      embed.FS
}

type PGRepository struct {
	db           PgxPoolIface
	queryBuilder sq.StatementBuilderType
}

func (config PGConfig) GetConnectionString(env string) string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=marble sslmode=disable", config.Hostname, config.User, config.Password)
	if env == "DEV" {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}

func New(env string, PGConfig PGConfig) (*PGRepository, error) {
	connectionString := PGConfig.GetConnectionString(env)

	dbpool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	var currentPGUser string
	err = dbpool.QueryRow(context.Background(), "SELECT current_user;").Scan(&currentPGUser)
	if err != nil {
		return nil, fmt.Errorf("unable to get current user: %w", err)
	}

	var searchPath string
	err = dbpool.QueryRow(context.Background(), "SHOW search_path;").Scan(&searchPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get search path: %w", err)
	}

	r := &PGRepository{
		db:           dbpool,
		queryBuilder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	return r, nil
}

func RunMigrations(env string, pgConfig PGConfig, migrationsDirectory string, logger *slog.Logger) {
	connectionString := pgConfig.GetConnectionString(env)

	migrationDB, err := sql.Open("pgx", connectionString)
	defer migrationDB.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// start goose migrations
	logger.Info("Migrations starting")
	goose.SetBaseFS(pgConfig.MigrationFS)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	err = migrationDB.Ping()
	if err != nil {
		logger.Error("Unable to ping database: \n" + err.Error())
		panic(err)
	}

	if err := goose.Up(migrationDB, migrationsDirectory); err != nil {
		logger.Error("unable to run migrations: \n" + err.Error())
		panic(err)
	}
}
