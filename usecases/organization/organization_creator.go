package organization

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type OrganizationCreator struct {
	TransactionFactory     executor_factory.TransactionFactory
	OrganizationRepository repositories.OrganizationRepository
	DataModelRepository    repositories.DataModelRepository
	OrganizationSeeder     OrganizationSeeder
}

func (creator *OrganizationCreator) CreateOrganizationWithId(
	ctx context.Context,
	newOrganizationId, name string,
) (models.Organization, error) {
	organization, err := executor_factory.TransactionReturnValue(ctx,
		creator.TransactionFactory, func(tx repositories.Executor) (models.Organization, error) {
			if err := creator.OrganizationRepository.CreateOrganization(ctx, tx, newOrganizationId, name); err != nil {
				return models.Organization{}, err
			}
			organization, err := creator.OrganizationRepository.GetOrganizationById(ctx, tx, newOrganizationId)
			if err != nil {
				return models.Organization{}, err
			}

			// create entry in organizations_schema
			if err := creator.OrganizationRepository.CreateOrganizationSchema(
				ctx,
				tx,
				organization.Id,
				fmt.Sprintf("org-%s", organization.DatabaseName),
			); err != nil {
				return models.Organization{}, err
			}
			return organization, nil
		})
	if err != nil {
		return models.Organization{}, err
	}

	if err = creator.OrganizationSeeder.Seed(ctx, organization.Id); err != nil {
		return models.Organization{}, err
	}

	return organization, nil
}
