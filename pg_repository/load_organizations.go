package pg_repository

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
	"gitlab.com/marble5/marble-backend-are-poc/app"
)

func (r *PGRepository) LoadOrganizations() {

	///////////////////////////////
	// Organizations
	///////////////////////////////

	// limit variables scope
	{
		rows, _ := r.db.Query(context.Background(), "SELECT id, name FROM organizations")

		var id, name string
		_, err := pgx.ForEachRow(rows, []any{&id, &name}, func() error {

			// Create organization
			r.organizations[id] = &app.Organization{
				ID:   id,
				Name: name,

				Tokens: make(map[string]string),
			}

			return nil
		})

		if err != nil {
			log.Printf("Error getting organizations: %v\n", err)
		}
	}

	///////////////////////////////
	// Tokens
	///////////////////////////////

	{
		rows, _ := r.db.Query(context.Background(), "SELECT id, org_id, token FROM tokens")

		var id, orgID, token string
		_, err := pgx.ForEachRow(rows, []any{&id, &orgID, &token}, func() error {

			// Add token to organizations
			r.organizations[orgID].Tokens[id] = token

			return nil
		})

		if err != nil {
			log.Printf("Error getting organization tokens: %v\n", err)
		}

	}

	///////////////////////////////
	// Inject data models & scenario directly in-memory
	///////////////////////////////

	{
		testClientName := "Test organization"

		var testOrgID string
		err := r.db.QueryRow(context.Background(), "SELECT id FROM organizations WHERE name = $1;", testClientName).Scan(&testOrgID)
		if err != nil {
			log.Printf("unable to get test org ID: %v", err)
		}
		log.Printf("test client: %v (# %v)\n", testClientName, testOrgID)

		r.FillOrgWithTestData(testOrgID)
	}

	///////////////////////////////
	// Data models
	///////////////////////////////

	{
		//TODO
	}

	///////////////////////////////
	// Scenarios
	///////////////////////////////

	{
		//TODO
	}

}
