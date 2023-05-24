package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type OrganizationUseCase struct {
	organizationRepository repositories.OrganizationRepository
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	return usecase.organizationRepository.GetOrganizations(ctx)
}

func (usecase *OrganizationUseCase) CreateOrganization(ctx context.Context, createOrga models.CreateOrganizationInput) (models.Organization, error) {
	return usecase.organizationRepository.CreateOrganization(ctx, createOrga)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationID string) (models.Organization, error) {
	return usecase.organizationRepository.GetOrganization(ctx, organizationID)
}

func (usecase *OrganizationUseCase) UpdateOrganization(ctx context.Context, organization models.UpdateOrganizationInput) (models.Organization, error) {
	return usecase.organizationRepository.UpdateOrganization(ctx, organization)
}

func (usecase *OrganizationUseCase) SoftDeleteOrganization(ctx context.Context, organizationID string) error {
	return usecase.organizationRepository.SoftDeleteOrganization(ctx, organizationID)
}
