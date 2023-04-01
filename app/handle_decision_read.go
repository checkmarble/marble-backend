package app

import "marble/marble-backend/app/scenarios"

func (a *App) GetDecision(orgID string, decisionID string) (scenarios.Decision, error) {

	return a.repository.GetDecision(orgID, decisionID)

}
