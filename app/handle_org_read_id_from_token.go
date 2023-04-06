package app

import "context"

func (a *App) GetOrganizationIDFromToken(token string) (orgID string, err error) {
	return a.repository.GetOrganizationIDFromToken(context.TODO(), token)
}
