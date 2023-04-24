package app

import "context"

func (app *App) GetOrganizationIDFromToken(ctx context.Context, token string) (orgID string, err error) {
	return app.repository.GetOrganizationIDFromToken(ctx, token)
}
