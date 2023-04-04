package pg_repository

import (
	"context"
	"log"
	"time"

	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	"github.com/jackc/pgx/v5"
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

		///////////////////////////////
		// Create and store a scenario
		///////////////////////////////
		rules := []app.Rule{
			{
				Formula:       &operators.True{},
				ScoreModifier: 2,
				Name:          "Rule 1 Name",
				Description:   "Rule 1 Desc",
			},
			{
				Formula:       &operators.False{},
				ScoreModifier: 2,
				Name:          "Rule 2 Name",
				Description:   "Rule 2 Desc",
			},
			{
				Formula:       &operators.EqBool{Left: &operators.True{}, Right: &operators.True{}},
				ScoreModifier: 2,
				Name:          "Rule 3 Name",
				Description:   "Rule 3 Desc",
			},
			{
				Formula:       &operators.EqBool{Left: &operators.True{}, Right: &operators.EqBool{Left: &operators.False{}, Right: &operators.False{}}},
				ScoreModifier: 2,
				Name:          "Rule 4 Name",
				Description:   "Rule 4 Desc",
			},
		}

		sib := app.ScenarioIterationBody{
			TriggerCondition: &operators.True{},
			Rules:            rules,

			ScoreReviewThreshold: 10,
			ScoreRejectThreshold: 30,
		}

		si := app.ScenarioIteration{
			ID: "2c5e8eab-a6ab-4e22-8992-ac8edd608bef", // same as scenarioID, no worries

			ScenarioID: "3a6cabee-a565-42b2-af40-5295386c8269",
			Version:    1,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Body:       sib,
		}

		s, err := r.CreateScenarioIteration(testOrgID, si)
		if err != nil {
			log.Printf("error creating scenario: %v", err)
		}
		err = r.PublishScenarioIteration(testOrgID, "3a6cabee-a565-42b2-af40-5295386c8269", s.ID)
		if err != nil {
			log.Printf("error creating scenario iteration: %v", err)
		}

		///////////////////////////////
		// Create and sotre (in-memory) a data model
		///////////////////////////////
		r.organizations[testOrgID].DataModel = app.DataModel{
			Tables: map[string]app.Table{
				"tx": {Name: "tx",
					Fields: map[string]app.Field{
						"id": {
							DataType: app.String,
						},
						"amount": {
							DataType: app.Float,
						},
						"sender_id": {
							DataType: app.String,
						},
					},
					LinksToSingle: map[string]app.LinkToSingle{
						"sender": {
							LinkedTableName: "user",
							ParentFieldName: "sender_id",
							ChildFieldName:  "id",
						},
					},
				},
				"transactions": {
					Name: "transactions",
					Fields: map[string]app.Field{
						"object_id": {
							DataType: app.String,
						},
						"updated_at":  {DataType: app.Timestamp},
						"value":       {DataType: app.Float},
						"title":       {DataType: app.String},
						"description": {DataType: app.String},
					},
					LinksToSingle: map[string]app.LinkToSingle{},
				},
				"user": {
					Name: "user",
					Fields: map[string]app.Field{
						"id": {
							DataType: app.String,
						},
						"name": {
							DataType: app.String,
						},
					},
				},
			},
		}

	}

}
