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

func (p *PopulateClientTables) CreateClientTables(marbleTx repositories.Transaction, organization models.Organization, database models.Database) error {

	// create entry in client_tables
	err := p.ClientTablesRepository.CreateClientTables(marbleTx, models.ClientTables{
		OrganizationId: organization.ID,
		Schema:         organization.DatabaseName,
	})
	if err != nil {
		return err
	}

	// Open a new transaction 'clientTx' to write in the client database.
	// The client can be in another sql instance
	// Note that the error is returned, so in case of a roolback in 'clientTx', 'marbleTx' will also be rolled back.
	return p.TransactionFactory.Transaction(database, func(clientTx repositories.Transaction) error {
		return p.ClientTablesRepository.CreateSchema(clientTx, organization.DatabaseName)
	})

}
