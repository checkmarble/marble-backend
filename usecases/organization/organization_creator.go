package organization

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type OrganizationSeeder interface {
	Seed(organizationId string) error
}

type OrganizationCreator struct {
	TransactionFactory     repositories.TransactionFactory
	OrganizationRepository repositories.OrganizationRepository
	OrganizationSeeder     OrganizationSeeder
	PopulateClientTables   PopulateClientTables
}

func (creator *OrganizationCreator) CreateOrganizationWithId(newOrganizationId string, createOrga models.CreateOrganizationInput) (models.Organization, error) {

	organization, err := repositories.TransactionReturnValue(creator.TransactionFactory, models.DATABASE_MARBLE, func(tx repositories.Transaction) (models.Organization, error) {
		err := creator.OrganizationRepository.CreateOrganization(tx, createOrga, newOrganizationId)
		if err != nil {
			return models.Organization{}, err
		}
		return creator.OrganizationRepository.GetOrganizationById(tx, newOrganizationId)
	})

	if err != nil {
		return models.Organization{}, err
	}

	err = creator.OrganizationSeeder.Seed(organization.ID)
	if err != nil {
		return models.Organization{}, err
	}

	err = creator.PopulateClientTables.CreateClientTables(organization, models.DATABASE_MARBLE)
	if err != nil {
		return models.Organization{}, err
	}

	return organization, nil
}
