package pg_repository

import (
	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/app/scenarios"
	"time"
)

func (r *PGRepository) FillOrgWithTestData(orgID string) {

	///////////////////////////////
	// Fill r.DataModel and r.Scenarios
	///////////////////////////////

	///////////////////////////////
	// Data model
	///////////////////////////////

	dm := data_model.DataModel{
		Tables: map[string]data_model.Table{
			"tx": {Name: "tx",
				Fields: map[string]data_model.Field{
					"id": {
						DataType: data_model.String,
					},
					"amount": {
						DataType: data_model.Float,
					},
					"sender_id": {
						DataType: data_model.String,
					},
				},
				LinksToSingle: map[string]data_model.LinkToSingle{
					"sender": {
						LinkedTableName: "user",
						ParentFieldName: "sender_id",
						ChildFieldName:  "id",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]data_model.Field{
					"object_id": {
						DataType: data_model.String,
					},
					"updated_at":  {DataType: data_model.Timestamp},
					"value":       {DataType: data_model.Float},
					"title":       {DataType: data_model.String},
					"description": {DataType: data_model.String},
				},
				LinksToSingle: map[string]data_model.LinkToSingle{},
			},
			"user": {
				Name: "user",
				Fields: map[string]data_model.Field{
					"id": {
						DataType: data_model.String,
					},
					"name": {
						DataType: data_model.String,
					},
				},
			},
		},
	}

	// map[clientID]datamodel
	// r.dataModels[orgID] = dm

	///////////////////////////////
	// Scenarios
	///////////////////////////////

	// Basic logical
	rules := []scenarios.Rule{
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

	///////////////////////////////
	// Create iteration & body
	///////////////////////////////

	sib := scenarios.ScenarioIterationBody{
		TriggerCondition: &operators.True{},
		Rules:            rules,

		ScoreReviewThreshold: 10,
		ScoreRejectThreshold: 30,
	}

	si := scenarios.ScenarioIteration{
		ID: "2c5e8eab-a6ab-4e22-8992-ac8edd608bef", // same as scenarioID, no worries

		ScenarioID: "3a6cabee-a565-42b2-af40-5295386c8269",
		Version:    1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Body:       sib,
	}

	// map[scenarioID]app.Scenario
	s := make(map[string]scenarios.Scenario)
	s["3a6cabee-a565-42b2-af40-5295386c8269"] = scenarios.Scenario{
		ID:          "3a6cabee-a565-42b2-af40-5295386c8269",
		Name:        "My 1st scenario",
		Description: "check if the API works",

		CreatedAt:         time.Now(),
		TriggerObjectType: "tx",

		LiveVersion: &si,
	}

	// map[clientID]map[scenarioID]app.Scenario
	// r.scenarios[orgID] = s

	///////////////////////////////
	// Fill r.Organizations
	///////////////////////////////

	r.organizations[orgID].Scenarios = s
	r.organizations[orgID].DataModel = dm

	// Ignore error, for duplicate insert
	r.PostScenario(orgID, s["3a6cabee-a565-42b2-af40-5295386c8269"])
}
