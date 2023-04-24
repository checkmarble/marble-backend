package pg_repository

import (
	"context"
	"log"

	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/utils"
)

func (r *PGRepository) Seed() {

	///////////////////////////////
	// Organizations
	///////////////////////////////

	_, err := r.CreateOrganization(context.TODO(), app.CreateOrganizationInput{
		Name:         "Marble",
		DatabaseName: "marble",
	})
	if err != nil {
		log.Printf("error creating organization: %v", err)
	}
	org, err := r.CreateOrganization(context.TODO(), app.CreateOrganizationInput{
		Name:         "Test organization",
		DatabaseName: "test_1",
	})
	if err != nil {
		log.Printf("error creating organization: %v", err)
	}

	///////////////////////////////
	// Tokens
	///////////////////////////////

	token, err := r.CreateToken(context.TODO(), CreateToken{
		OrgID: org.ID,
		Token: "token12345",
	})
	if err != nil {
		log.Printf("error creating token: %v", err)
	}

	///////////////////////////////
	// Create and sotre a data model
	///////////////////////////////
	r.CreateDataModel(context.TODO(), org.ID, app.DataModel{
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
					"updated_at":      {DataType: app.Timestamp},
					"value":           {DataType: app.Float},
					"title":           {DataType: app.String},
					"description":     {DataType: app.String},
					"bank_account_id": {DataType: app.String},
				},
				LinksToSingle: map[string]app.LinkToSingle{
					"bank_account": {
						LinkedTableName: "bank_accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "bank_account_id"},
				},
			},
			"bank_accounts": {
				Name: "bank_accounts",
				Fields: map[string]app.Field{
					"object_id": {
						DataType: app.String,
					},
					"updated_at": {DataType: app.Timestamp},
					"balance":    {DataType: app.Float},
					"name":       {DataType: app.String},
					"currency":   {DataType: app.String},
				},
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
	})
	///////////////////////////////
	// Create and store a scenario
	///////////////////////////////
	createScenarioInput := app.CreateScenarioInput{
		Name:              "test name",
		Description:       "test description",
		TriggerObjectType: "tx",
	}
	scenario, err := r.CreateScenario(context.TODO(), org.ID, createScenarioInput)
	if err != nil {
		log.Printf("error creating scenario: %v", err)
	}

	createScenarioIterationInput := app.CreateScenarioIterationInput{
		ScenarioID: scenario.ID,
		Body: &app.CreateScenarioIterationBody{
			TriggerCondition:     &operators.True{},
			ScoreReviewThreshold: utils.Ptr(10),
			ScoreRejectThreshold: utils.Ptr(30),
			Rules: []app.CreateRuleInput{
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
			},
		},
	}

	scenarioIteration, err := r.CreateScenarioIteration(context.TODO(), org.ID, createScenarioIterationInput)
	if err != nil {
		log.Printf("error creating scenario iteration: %v", err)
	}
	_, err = r.CreateScenarioPublication(context.TODO(), org.ID, app.CreateScenarioPublicationInput{
		ScenarioID:          scenarioIteration.ScenarioID,
		ScenarioIterationID: scenarioIteration.ID,
		PublicationAction:   app.Publish,
	})
	if err != nil {
		log.Printf("error publishind scenario iteration: %v", err)
	}

	log.Println("")
	log.Println("Finish to Seed the DB :")
	log.Printf("\t- $SCENARIO_ID: %s\n", scenario.ID)
	log.Printf("\t- $TOKEN: %s\n", token.Token)
	log.Println("")
}
