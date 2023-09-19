package organization

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type PopulateOrganizationSchema struct {
	TransactionFactory           transaction.TransactionFactory
	OrganizationRepository       repositories.OrganizationRepository
	OrganizationSchemaRepository repositories.OrganizationSchemaRepository
	DataModelRepository          repositories.DataModelRepository
}

func (p *PopulateOrganizationSchema) CreateOrganizationSchema(marbleTx repositories.Transaction, organization models.Organization, database models.Database) error {

	orgDatabaseSchema := models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   database,
		Schema:     fmt.Sprintf("org-%s", organization.DatabaseName),
	}
	// create entry in organizations_schema
	err := p.OrganizationSchemaRepository.CreateOrganizationSchema(marbleTx, models.OrganizationSchema{
		OrganizationId: organization.Id,
		DatabaseSchema: orgDatabaseSchema,
	})
	if err != nil {
		return err
	}

	dataModel, err := p.DataModelRepository.GetDataModel(marbleTx, organization.Id)
	if err != nil {
		return err
	}

	// Open a new transaction 'clientTx' to write in the client database.
	// The client can be in another sql instance
	// Note that the error is returned, so in case of a roolback in 'clientTx', 'marbleTx' will also be rolled back.
	return p.TransactionFactory.Transaction(orgDatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		return p.populate(orgSchemaTx, orgDatabaseSchema, dataModel)
	})
}

func (p *PopulateOrganizationSchema) WipeAndCreateOrganizationSchema(marbleTx repositories.Transaction, organizationId string, newDataModel models.DataModel) error {

	// fetch organization schema
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(marbleTx, organizationId)
	if err != nil {
		return err
	}

	// delete and recreate the entire postgres schema
	return p.TransactionFactory.Transaction(orgSchema.DatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		if err := p.OrganizationSchemaRepository.DeleteSchema(orgSchemaTx, orgSchema.DatabaseSchema.Schema); err != nil {
			return err
		}
		return p.populate(orgSchemaTx, orgSchema.DatabaseSchema, newDataModel)
	})
}

func (p *PopulateOrganizationSchema) populate(orgSchemaTx repositories.Transaction, databaseSchema models.DatabaseSchema, dataModel models.DataModel) error {

	err := p.OrganizationSchemaRepository.CreateSchema(orgSchemaTx, databaseSchema.Schema)
	if err != nil {
		return err
	}

	for _, table := range dataModel.Tables {
		err := p.OrganizationSchemaRepository.CreateTable(orgSchemaTx, databaseSchema.Schema, table)
		if err != nil {
			return err
		}
	}
	return nil
}
