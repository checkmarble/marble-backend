package app

import "context"

func (app *App) GetOrganizations(ctx context.Context) ([]Organization, error) {
	return app.repository.GetOrganizations(ctx)
}

func (app *App) CreateOrganization(ctx context.Context, organization CreateOrganizationInput) (Organization, error) {
	return app.repository.CreateOrganization(ctx, organization)
}

func (app *App) GetOrganization(ctx context.Context, organizationID string) (Organization, error) {
	return app.repository.GetOrganization(ctx, organizationID)
}

func (app *App) UpdateOrganization(ctx context.Context, organization UpdateOrganizationInput) (Organization, error) {
	return app.repository.UpdateOrganization(ctx, organization)
}

func (app *App) SoftDeleteOrganization(ctx context.Context, organizationID string) error {
	return app.repository.SoftDeleteOrganization(ctx, organizationID)
}
