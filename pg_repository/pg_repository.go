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
	"gitlab.com/marble5/marble-backend-are-poc/app"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PGRepository struct {
	// Postgres
	db *pgxpool.Pool // connection pool

	// in-memory data
	dataModels map[string]app.DataModel           //map[orgID]DataModel
	scenarios  map[string]map[string]app.Scenario //map[orgID][scenarioID]Scenario

	organizations map[string]*app.Organization // //map[orgID]Organization
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

	dm := make(map[string]app.DataModel)
	s := make(map[string]map[string]app.Scenario)
	o := make(map[string]*app.Organization)

	r := &PGRepository{
		db: dbpool,

		dataModels:    dm,
		scenarios:     s,
		organizations: o,
	}

	///////////////////////////////
	// Load organizations into memory
	///////////////////////////////

	r.LoadOrganizations()
	//r.Fill()

	//log.Println("Dumping organizations")
	//spew.Dump(r.organizations)

	r.Describe()

	return r, nil
}

func (r *PGRepository) Describe() {

	log.Println("organizations loaded:")
	for _, o := range r.organizations {
		log.Printf("%s (# %v)", o.Name, o.ID)

		log.Printf("\tscenarios\n")
		for _, s := range o.Scenarios {
			log.Printf("\t\t%s (# %v) : %s", s.Name, s.ID, s.Description)
		}

		log.Printf("\ttokens\n")
		for _, t := range o.Tokens {
			log.Printf("\t\t%s", t)
		}

		log.Println()
	}
}

func (r *PGRepository) GetDataModel(orgID string) (app.DataModel, error) {
	org, orgFound := r.organizations[orgID]
	if !orgFound {
		return app.DataModel{}, app.ErrNotFoundInRepository
	}
	return org.DataModel, nil
}

func (r *PGRepository) GetScenario(orgID string, scenarioID string) (app.Scenario, error) {

	org, orgFound := r.organizations[orgID]
	if !orgFound {
		return app.Scenario{}, app.ErrNotFoundInRepository
	}

	scenario, scenarioFound := org.Scenarios[scenarioID]
	if !scenarioFound {
		return app.Scenario{}, app.ErrNotFoundInRepository
	}

	return scenario, nil
}

func (r *PGRepository) GetOrganizationIDFromToken(token string) (orgID string, err error) {
	for _, o := range r.organizations {
		for _, t := range o.Tokens {
			if t == token {
				return o.ID, nil
			}
		}
	}

	return "", app.ErrNotFoundInRepository
}
