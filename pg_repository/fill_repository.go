package pg_repository

import "marble/marble-backend/app"

func (r *PGRepository) FillOrgWithTestData(orgID string) {

	///////////////////////////////
	// Fill r.DataModel and r.Scenarios
	///////////////////////////////

	///////////////////////////////
	// Data model
	///////////////////////////////

	dm := app.DataModel{
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
					"id": {DataType: app.String},
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

	// map[clientID]datamodel
	// r.dataModels[orgID] = dm

	///////////////////////////////
	// Scenarios
	///////////////////////////////

	// Basic logical
	rules := []app.Rule{
		{
			RootNode:      app.And{Left: app.True{}, Right: app.True{}},
			ScoreModifier: 2,
			Name:          "Rule 1 Name",
			Description:   "Rule 1 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.False{}},
			ScoreModifier: 2,
			Name:          "Rule 2 Name",
			Description:   "Rule 2 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.IntValue{Value: 5}, Right: app.IntValue{Value: 5}}}},
			ScoreModifier: 2,
			Name:          "Rule 3 Name",
			Description:   "Rule 3 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.IntValue{Value: 6}, Right: app.IntValue{Value: 5}}}},
			ScoreModifier: 2,
			Name:          "Rule 4 Name",
			Description:   "Rule 4 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.FloatValue{Value: 5}, Right: app.IntValue{Value: 5}}}},
			ScoreModifier: 2,
			Name:          "Rule 5 Name",
			Description:   "Rule 5 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.FloatValue{Value: 5}, Right: app.FieldValue{Datamodel: dm, RootTableName: "tx", Path: []string{"amount"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 6 Name",
			Description:   "Rule 6 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.FloatValue{Value: 6}, Right: app.FieldValue{Datamodel: dm, RootTableName: "tx", Path: []string{"amount"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 7 Name",
			Description:   "Rule 7 Desc",
		},
		{
			RootNode:      app.And{Left: app.True{}, Right: app.And{Left: app.True{}, Right: app.Eq{Left: app.FloatValue{Value: 6}, Right: app.FieldValue{Datamodel: dm, RootTableName: "tx", Path: []string{"sender"}}}}},
			ScoreModifier: 2,
			Name:          "Rule 8 Name",
			Description:   "Rule 8 Desc",
		},
	}

	// map[scenarioID]app.Scenario
	s := make(map[string]app.Scenario)
	s["3a6cabee-a565-42b2-af40-5295386c8269"] = app.Scenario{
		ID: "3a6cabee-a565-42b2-af40-5295386c8269",

		Name:        "My 1st scenario",
		Description: "check if the API works",
		Version:     "alpha",

		TriggerCondition:  app.True{},
		Rules:             rules,
		TriggerObjectType: "tx",

		OutcomeApproveScore: 10,
		OutcomeRejectScore:  30,
	}

	// map[clientID]map[scenarioID]app.Scenario
	// r.scenarios[orgID] = s

	///////////////////////////////
	// Fill r.Organizations
	///////////////////////////////

	r.organizations[orgID].Scenarios = s
	r.organizations[orgID].DataModel = dm
}
