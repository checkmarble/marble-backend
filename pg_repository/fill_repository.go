package pg_repository

import (
	"marble/marble-backend/app"
	"time"
)

func (r *PGRepository) FillOrgWithTestData(orgID string) {

	///////////////////////////////
	// Fill r.DataModel and r.Scenarios
	///////////////////////////////

	///////////////////////////////
	// Data model
	///////////////////////////////

	dm := app.DataModel{
		Tables: map[string]app.Table{
			"tx": {
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
			"user": {
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

	// map[clientID]datamodel
	// r.dataModels[orgID] = dm

	///////////////////////////////
	// Scenarios
	///////////////////////////////

	// Basic logical
	rules := []app.Rule{
		{
			RootNode:      app.And{app.True{}, app.True{}},
			ScoreModifier: 2,
			Name:          "Rule 1 Name",
			Description:   "Rule 1 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.False{}},
			ScoreModifier: 2,
			Name:          "Rule 2 Name",
			Description:   "Rule 2 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.IntValue{5}, app.IntValue{5}}}},
			ScoreModifier: 2,
			Name:          "Rule 3 Name",
			Description:   "Rule 3 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.IntValue{6}, app.IntValue{5}}}},
			ScoreModifier: 2,
			Name:          "Rule 4 Name",
			Description:   "Rule 4 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.FloatValue{5}, app.IntValue{5}}}},
			ScoreModifier: 2,
			Name:          "Rule 5 Name",
			Description:   "Rule 5 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.FloatValue{5}, app.FieldValue{dm, "tx", []string{"amount"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 6 Name",
			Description:   "Rule 6 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.FloatValue{6}, app.FieldValue{dm, "tx", []string{"amount"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 7 Name",
			Description:   "Rule 7 Desc",
		},
		{
			RootNode:      app.And{app.True{}, app.And{app.True{}, app.Eq{app.FloatValue{6}, app.FieldValue{dm, "tx", []string{"sender"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 8 Name",
			Description:   "Rule 8 Desc",
		},
	}

	///////////////////////////////
	// Create iteration & body
	///////////////////////////////

	sib := app.ScenarioIterationBody{
		TriggerCondition: app.True{},
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

	// map[scenarioID]app.Scenario
	s := make(map[string]app.Scenario)
	s["3a6cabee-a565-42b2-af40-5295386c8269"] = app.Scenario{
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
}
