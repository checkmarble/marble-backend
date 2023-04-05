package app

import "context"

func (a *App) GetScenarios(organizationID string) ([]Scenario, error) {
	return a.repository.GetScenarios(context.TODO(), organizationID)
}

func (a *App) CreateScenario(organizationID string, scenario Scenario) (Scenario, error) {
	return a.repository.PostScenario(context.TODO(), organizationID, scenario)
}

func (a *App) GetScenario(organizationID string, scenarioID string) (Scenario, error) {
	return a.repository.GetScenario(context.TODO(), organizationID, scenarioID)
}
