package app

import "context"

func (a *App) GetDecision(orgID string, decisionID string) (Decision, error) {

	return a.repository.GetDecision(context.TODO(), orgID, decisionID)

}
