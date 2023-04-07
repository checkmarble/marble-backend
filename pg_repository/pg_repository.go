package pg_repository

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PGRepository struct {
	db           *pgxpool.Pool // connection pool
	queryBuilder squirrel.StatementBuilderType
}

func New(host string, port string, user string, password string, migrationFS embed.FS) (*PGRepository, error) {
	connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=marble sslmode=disable", host, port, user, password)
	log.Printf("connection string: %v\n", connectionString)

	///////////////////////////////
	// Run migrations if any
	// This requires its own *sql.DB connection
	///////////////////////////////

	// Setup its own connection
	migrationDB, err := sql.Open("pgx", connectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// start goose migrations
	log.Println("Migrations starting")

	goose.SetBaseFS(migrationFS)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(migrationDB, "migrations"); err != nil {
		panic(err)
	}

	migrationDB.Close()
	log.Println("Migrations completed")

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
