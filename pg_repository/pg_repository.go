package pg_repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

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
	Database         string
	ConnectionString string
}

type PGRepository struct {
	db           *pgxpool.Pool
	queryBuilder sq.StatementBuilderType
}

func (config PGConfig) GetConnectionString(env string) string {
	if config.ConnectionString != "" {
		return config.ConnectionString
	}
	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable", config.Hostname, config.User, config.Password, config.Database)
	if env == "DEV" {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, config.Port)
	}
	return connectionString
}

func New(dbpool *pgxpool.Pool) (*PGRepository, error) {

	// Test connection
	var currentPGUser string
	err := dbpool.QueryRow(context.Background(), "SELECT current_user;").Scan(&currentPGUser)
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

func (r *PGRepository) GetDbPool() *pgxpool.Pool {
	return r.db
}
