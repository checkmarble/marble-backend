package db_executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type transactionFactoryRepository interface {
	GetExecutor(databaseSchema models.DatabaseSchema) *repositories.ExecutorPostgres
	Transaction(ctx context.Context, databaseSchema models.DatabaseSchema, fn func(tx *repositories.ExecutorPostgres) error) error
}

type DbExecutorFactory struct {
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	transactionFactoryRepository transactionFactoryRepository
}

func NewDbExecutorFactory(
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	transactionFactoryRepository transactionFactoryRepository,
) *DbExecutorFactory {
	return &DbExecutorFactory{
		organizationSchemaRepository: organizationSchemaRepository,
		transactionFactoryRepository: transactionFactoryRepository,
	}
}

func (factory *DbExecutorFactory) organizationDatabaseSchema(
	ctx context.Context,
	organizationId string,
) (models.DatabaseSchema, error) {
	organizationSchema, err := factory.organizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, nil, organizationId)
	if err != nil {
		return models.DatabaseSchema{}, err
	}

	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   organizationSchema.DatabaseSchema.Database,
		Schema:     organizationSchema.DatabaseSchema.Schema,
	}, nil
}

func (factory *DbExecutorFactory) TransactionInOrgSchema(
	ctx context.Context,
	organizationId string,
	f func(tx *repositories.ExecutorPostgres) error,
) error {
	dbSchema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return err
	}

	return factory.transactionFactoryRepository.Transaction(ctx, dbSchema, f)
}

func (factory *DbExecutorFactory) Transaction(
	ctx context.Context,
	f func(tx *repositories.ExecutorPostgres) error,
) error {
	return factory.transactionFactoryRepository.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, f)
}

func (factory *DbExecutorFactory) NewClientDbExecutor(
	ctx context.Context,
	organizationId string,
) (*repositories.ExecutorPostgres, error) {
	schema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return &repositories.ExecutorPostgres{}, err
	}

	return factory.transactionFactoryRepository.GetExecutor(schema), nil
}
