package organization

import (
	"context"
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

func (p *PopulateOrganizationSchema) CreateOrganizationSchema(ctx context.Context, marbleTx repositories.Transaction, organization models.Organization, database models.Database) error {

	orgDatabaseSchema := models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   database,
		Schema:     fmt.Sprintf("org-%s", organization.DatabaseName),
	}
	// create entry in organizations_schema
	return p.OrganizationSchemaRepository.CreateOrganizationSchema(ctx, marbleTx, models.OrganizationSchema{
		OrganizationId: organization.Id,
		DatabaseSchema: orgDatabaseSchema,
	})
}

func (p *PopulateOrganizationSchema) CreateTable(ctx context.Context, marbleTx repositories.Transaction, organizationId, tableName string) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, marbleTx, organizationId)
	if err != nil {
		return err
	}

	return p.TransactionFactory.Transaction(ctx, orgSchema.DatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		err := p.OrganizationSchemaRepository.CreateSchema(ctx, orgSchemaTx, orgSchema.DatabaseSchema.Schema)
		if err != nil {
			return err
		}
		return p.OrganizationSchemaRepository.CreateTable(ctx, orgSchemaTx, orgSchema.DatabaseSchema.Schema, tableName)
	})
}

func (p *PopulateOrganizationSchema) CreateField(ctx context.Context, marbleTx repositories.Transaction, organizationID, tableName string, field models.DataModelField) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, marbleTx, organizationID)
	if err != nil {
		return err
	}

	return p.TransactionFactory.Transaction(ctx, orgSchema.DatabaseSchema, func(orgSchemaTx repositories.Transaction) error {
		return p.OrganizationSchemaRepository.CreateField(ctx, orgSchemaTx, orgSchema.DatabaseSchema.Schema, tableName, field)
	})
}
