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
	return p.OrganizationSchemaRepository.CreateOrganizationSchema(marbleTx, models.OrganizationSchema{
		OrganizationId: organization.Id,
		DatabaseSchema: orgDatabaseSchema,
	})
}

func (p *PopulateOrganizationSchema) CreateTable(marbleTx repositories.Transaction, organizationId, tableName string) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(marbleTx, organizationId)
	if err != nil {
		return err
	}

	return p.TransactionFactory.Transaction(orgSchema.DatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		err := p.OrganizationSchemaRepository.CreateSchema(orgSchemaTx, orgSchema.DatabaseSchema.Schema)
		if err != nil {
			return err
		}
		return p.OrganizationSchemaRepository.CreateTable(orgSchemaTx, orgSchema.DatabaseSchema.Schema, tableName)
	})
}

func (p *PopulateOrganizationSchema) CreateField(marbleTx repositories.Transaction, organizationID, tableName string, field models.DataModelField) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(marbleTx, organizationID)
	if err != nil {
		return err
	}

	return p.TransactionFactory.Transaction(orgSchema.DatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		return p.OrganizationSchemaRepository.CreateField(orgSchemaTx, orgSchema.DatabaseSchema.Schema, tableName, field)
	})
}
