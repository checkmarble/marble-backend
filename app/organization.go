package app

import (
	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/scenarios"
)

type Organization struct {
	ID   string
	Name string

	Tokens    map[string]string //map[tokenID]token
	DataModel data_model.DataModel
	Scenarios map[string]scenarios.Scenario //map[scenarioID]Scenario
}
