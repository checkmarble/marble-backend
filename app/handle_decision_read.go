package app

func (a *App) GetDecision(orgID string, decisionID string) (Decision, error) {

	return a.repository.GetDecision(orgID, decisionID)

}
