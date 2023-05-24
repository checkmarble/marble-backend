package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type OrganizationRepository interface {
	GetOrganizations(ctx context.Context) ([]models.Organization, error)
	CreateOrganization(ctx context.Context, organization models.CreateOrganizationInput) (models.Organization, error)
	GetOrganization(ctx context.Context, organizationID string) (models.Organization, error)
	UpdateOrganization(ctx context.Context, organization models.UpdateOrganizationInput) (models.Organization, error)
	SoftDeleteOrganization(ctx context.Context, organizationID string) error
}
