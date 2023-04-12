package app

import "context"

func (a *App) GetOrganizations(ctx context.Context) ([]Organization, error) {
	return a.repository.GetOrganizations(ctx)
}

func (a *App) CreateOrganization(ctx context.Context, organization CreateOrganizationInput) (Organization, error) {
	return a.repository.CreateOrganization(ctx, organization)
}

func (a *App) GetOrganization(ctx context.Context, organizationID string) (Organization, error) {
	return a.repository.GetOrganization(ctx, organizationID)
}

func (a *App) UpdateOrganization(ctx context.Context, organization UpdateOrganizationInput) (Organization, error) {
	return a.repository.UpdateOrganization(ctx, organization)
}
