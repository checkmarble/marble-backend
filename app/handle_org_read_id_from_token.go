package app

import "context"

func (a *App) GetOrganizationIDFromToken(ctx context.Context, token string) (orgID string, err error) {
	return a.repository.GetOrganizationIDFromToken(ctx, token)
}
