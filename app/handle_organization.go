package app

import "context"

func (a *App) GetOrganizations(ctx context.Context) ([]Organization, error) {
	return a.repository.GetOrganizations(ctx)
}

func (a *App) CreateOrganization(ctx context.Context, organisation CreateOrganizationInput) (Organization, error) {
	return a.repository.CreateOrganization(ctx, organisation)
}

func (a *App) GetOrganization(ctx context.Context, organizationID string) (Organization, error) {
	return a.repository.GetOrganization(ctx, organizationID)
}
