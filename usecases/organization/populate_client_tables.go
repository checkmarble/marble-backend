package organization

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type PopulateClientTables struct {
	TransactionFactory     repositories.TransactionFactory
	OrganizationRepository repositories.OrganizationRepository
	ClientTablesRepository repositories.ClientTablesRepository
}

func (p *PopulateClientTables) CreateClientTables(organization models.Organization, database models.Database) error {

	// create entry in client_tables
	return p.TransactionFactory.Transaction(models.DATABASE_MARBLE, func(tx repositories.Transaction) error {
		err := p.ClientTablesRepository.CreateClientTables(tx, models.ClientTables{
			OrganizationId: organization.ID,
			Schema:         organization.DatabaseName,
		})
		if err != nil {
			return err
		}

		return p.ClientTablesRepository.CreateSchema(tx, organization.DatabaseName)
	})

}
