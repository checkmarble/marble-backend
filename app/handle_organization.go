package app

import "context"

type RepositoryOrganizations interface {
	GetOrganizations(ctx context.Context) ([]Organization, error)
	CreateOrganization(ctx context.Context, organization CreateOrganizationInput) (Organization, error)
	GetOrganization(ctx context.Context, orgID string) (Organization, error)
	UpdateOrganization(ctx context.Context, input UpdateOrganizationInput) (Organization, error)
	SoftDeleteOrganization(ctx context.Context, orgID string) error
}

func (app *App) GetOrganizations(ctx context.Context) ([]Organization, error) {
	return app.repository.GetOrganizations(ctx)
}

func (app *App) CreateOrganization(ctx context.Context, organization CreateOrganizationInput) (Organization, error) {
	return app.repository.CreateOrganization(ctx, organization)
}

func (app *App) GetOrganization(ctx context.Context, organizationID string) (Organization, error) {
	return app.repository.GetOrganization(ctx, organizationID)
}

func (app *App) UpdateOrganization(ctx context.Context, input UpdateOrganizationInput) (Organization, error) {
	return app.repository.UpdateOrganization(ctx, input)
}

func (app *App) SoftDeleteOrganization(ctx context.Context, organizationID string) error {
	return app.repository.SoftDeleteOrganization(ctx, organizationID)
}
