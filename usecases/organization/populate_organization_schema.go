package organization

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type PopulateOrganizationSchema struct {
	ExecutorFactory              executor_factory.ExecutorFactory
	OrganizationSchemaRepository repositories.OrganizationSchemaRepository
}

func (p *PopulateOrganizationSchema) CreateTable(ctx context.Context, exec repositories.Executor, organizationId, tableName string) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, exec, organizationId)
	if err != nil {
		return err
	}

	db, err := p.ExecutorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	if err := p.OrganizationSchemaRepository.CreateSchema(ctx, db,
		orgSchema.DatabaseSchema.Schema); err != nil {
		return err
	}

	return p.OrganizationSchemaRepository.CreateTable(ctx, db, orgSchema.DatabaseSchema.Schema, tableName)
}

func (p *PopulateOrganizationSchema) CreateField(ctx context.Context, tx repositories.Executor,
	organizationId, tableName string, field models.DataModelField,
) error {
	orgSchema, err := p.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, tx, organizationId)
	if err != nil {
		return err
	}

	db, err := p.ExecutorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return err
	}

	return p.OrganizationSchemaRepository.CreateField(ctx, db, orgSchema.DatabaseSchema.Schema, tableName, field)
}
