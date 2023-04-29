package pg_repository

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v5/stdlib" //https://github.com/jackc/pgx/wiki/Getting-started-with-pgx-through-database-sql
)

type PgxPoolIface interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Begin(context.Context) (pgx.Tx, error)
	Close()
}

type PGCOnfig struct {
	Hostname    string
	Port        string
	User        string
	Password    string
	MigrationFS embed.FS
}

type PGRepository struct {
	db           PgxPoolIface
	queryBuilder squirrel.StatementBuilderType
}

func New(env string, pgConfig PGCOnfig) (*PGRepository, error) {

	connectionString := fmt.Sprintf("host=%s user=%s password=%s database=marble sslmode=disable", pgConfig.Hostname, pgConfig.User, pgConfig.Password)
	if env == "DEV" {
		// Cloud Run connects to the DB through a proxy and a unix socket, so we don't need need to specify the port
		// but we do when running locally
		connectionString = fmt.Sprintf("%s port=%s", connectionString, pgConfig.Port)
	}

	///////////////////////////////
	// Run migrations if any
	///////////////////////////////

	// Setup its own connection
	migrationDB, err := sql.Open("pgx", connectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// start goose migrations
	log.Println("Migrations starting")

	goose.SetBaseFS(pgConfig.MigrationFS)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	err = migrationDB.Ping()
	if err != nil {
		log.Println("error while pinging db")
		panic(err)
	}

	if err := goose.Up(migrationDB, "migrations"); err != nil {
		panic(err)
	}

	migrationDB.Close()

	///////////////////////////////
	// Setup connection pool
	///////////////////////////////

	dbpool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		log.Printf("Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}

	log.Printf("DB connection pool created. Stats: %+v\n", dbpool.Stat())

	///////////////////////////////
	// Test connection
	///////////////////////////////
	var currentPGUser string
	err = dbpool.QueryRow(context.Background(), "SELECT current_user;").Scan(&currentPGUser)
	if err != nil {
		log.Printf("unable to get current user: %v", err)
	}
	log.Printf("current Postgres user: %v\n", currentPGUser)

	var searchPath string
	err = dbpool.QueryRow(context.Background(), "SHOW search_path;").Scan(&searchPath)
	if err != nil {
		log.Printf("unable to get search_path user: %v", err)
	}
	log.Printf("search path: %v\n", searchPath)

	///////////////////////////////
	// Build repository
	///////////////////////////////

	r := &PGRepository{
		db:           dbpool,
		queryBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}

	return r, nil
}
