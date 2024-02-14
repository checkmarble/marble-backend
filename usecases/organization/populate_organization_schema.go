package organization

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type PopulateOrganizationSchema struct {
	ClientSchemaExecutorFactory  executor_factory.ClientSchemaExecutorFactory
	OrganizationRepository       repositories.OrganizationRepository
	OrganizationSchemaRepository repositories.OrganizationSchemaRepository
	DataModelRepository          repositories.DataModelRepository
}

func (p *PopulateOrganizationSchema) CreateOrganizationSchema(ctx context.Context, exec repositories.Executor, organization models.Organization, database models.Database) error {

	orgDatabaseSchema := models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   database,
		Schema:     fmt.Sprintf("org-%s", organization.DatabaseName),
	}
	// create entry in organizations_schema
	return p.OrganizationSchemaRepository.CreateOrganizationSchema(ctx, exec, models.OrganizationSchema{
		OrganizationId: organization.Id,
		DatabaseSchema: orgDatabaseSchema,
	})
}

func (p *PopulateOrganizationSchema) CreateTable(ctx context.Context, exec repositories.Executor, organizationId, tableName string) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, exec, organizationId)
	if err != nil {
		return err
	}

	db, err := p.ClientSchemaExecutorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	if err := p.OrganizationSchemaRepository.CreateSchema(ctx, db, orgSchema.DatabaseSchema.Schema); err != nil {
		return err
	}

	return p.OrganizationSchemaRepository.CreateTable(ctx, db, orgSchema.DatabaseSchema.Schema, tableName)
}

func (p *PopulateOrganizationSchema) CreateField(ctx context.Context, tx repositories.Executor, organizationId, tableName string, field models.DataModelField) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, tx, organizationId)
	if err != nil {
		return err
	}

	db, err := p.ClientSchemaExecutorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	return p.OrganizationSchemaRepository.CreateField(ctx, db, orgSchema.DatabaseSchema.Schema, tableName, field)
}
