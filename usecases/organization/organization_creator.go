package organization

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type OrganizationCreator struct {
	TransactionFactory         transaction.TransactionFactory
	OrganizationRepository     repositories.OrganizationRepository
	DataModelRepository        repositories.DataModelRepository
	OrganizationSeeder         OrganizationSeeder
	PopulateOrganizationSchema PopulateOrganizationSchema
}

func (creator *OrganizationCreator) CreateOrganizationWithId(newOrganizationId string, createOrga models.CreateOrganizationInput) (models.Organization, error) {

	organization, err := transaction.TransactionReturnValue(creator.TransactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Organization, error) {
		if err := creator.OrganizationRepository.CreateOrganization(tx, createOrga, newOrganizationId); err != nil {
			return models.Organization{}, err
		}
		//if err := creator.createDataModel(tx, newOrganizationId); err != nil {
		//	return models.Organization{}, err
		//}
		return creator.OrganizationRepository.GetOrganizationById(tx, newOrganizationId)
	})

	if err != nil {
		return models.Organization{}, err
	}

	err = creator.OrganizationSeeder.Seed(organization.Id)
	if err != nil {
		return models.Organization{}, err
	}

	_, err = transaction.TransactionReturnValue(creator.TransactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (any, error) {
		// store client's data in marble DB
		orgDatabase := models.DATABASE_MARBLE
		err := creator.PopulateOrganizationSchema.CreateOrganizationSchema(tx, organization, orgDatabase)

		return nil, err
	})
	if err != nil {
		return models.Organization{}, err
	}

	return organization, nil
}
