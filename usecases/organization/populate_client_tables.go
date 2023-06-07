package organization

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type PopulateClientTables struct {
	TransactionFactory     repositories.TransactionFactory
	OrganizationRepository repositories.OrganizationRepository
	ClientTablesRepository repositories.ClientTablesRepository
	DataModelRepository    repositories.DataModelRepository
}

func (p *PopulateClientTables) CreateClientTables(marbleTx repositories.Transaction, organization models.Organization, database models.Database) error {

	// create entry in client_tables
	err := p.ClientTablesRepository.CreateClientTables(marbleTx, models.ClientTables{
		OrganizationId: organization.ID,
		Schema:         fmt.Sprintf("org-%s", organization.DatabaseName),
	})
	if err != nil {
		return err
	}

	// Open a new transaction 'clientTx' to write in the client database.
	// The client can be in another sql instance
	// Note that the error is returned, so in case of a roolback in 'clientTx', 'marbleTx' will also be rolled back.
	return p.TransactionFactory.Transaction(database, func(clientTx repositories.Transaction) error {
		schema := organization.DatabaseName
		err := p.ClientTablesRepository.CreateSchema(clientTx, schema)
		if err != nil {
			return err
		}

		dataModel, err := p.DataModelRepository.GetDataModel(context.TODO(), organization.ID)
		if err != nil {
			return err
		}
		for _, table := range dataModel.Tables {
			err := p.ClientTablesRepository.CreateTable(clientTx, schema, table)
			if err != nil {
				return err
			}
		}
		return nil
	})

}
