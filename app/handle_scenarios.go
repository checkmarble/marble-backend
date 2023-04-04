package app

func (a *App) GetScenarios(organizationID string) ([]Scenario, error) {
	return a.repository.GetScenarios(organizationID)
}

func (a *App) CreateScenario(organizationID string, scenario Scenario) (Scenario, error) {
	return a.repository.PostScenario(organizationID, scenario)
}

func (a *App) GetScenario(organizationID string, scenarioID string) (Scenario, error) {
	return a.repository.GetScenario(organizationID, scenarioID)
}
