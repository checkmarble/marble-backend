package app

import "context"

func (a *App) GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error) {
	return a.repository.GetDecision(ctx, orgID, decisionID)
}
