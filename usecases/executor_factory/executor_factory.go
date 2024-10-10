package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// interfaces used by the class
type executorFactoryRepository interface {
	GetExecutor(databaseSchema models.DatabaseSchema) repositories.Executor
	Transaction(ctx context.Context, databaseSchema models.DatabaseSchema,
		fn func(tx repositories.Transaction) error) error
}

type organizationSchemaReader interface {
	OrganizationSchemaOfOrganization(ctx context.Context, exec repositories.Executor,
		organizationId string) (models.OrganizationSchema, error)
}

type DbExecutorFactory struct {
	organizationSchemaReader     organizationSchemaReader
	transactionFactoryRepository executorFactoryRepository
}

func NewDbExecutorFactory(
	orgSchemaReader organizationSchemaReader,
	transactionFactoryRepository executorFactoryRepository,
) DbExecutorFactory {
	return DbExecutorFactory{
		organizationSchemaReader:     orgSchemaReader,
		transactionFactoryRepository: transactionFactoryRepository,
	}
}

func (factory DbExecutorFactory) organizationDatabaseSchema(
	ctx context.Context,
	organizationId string,
) (models.DatabaseSchema, error) {
	organizationSchema, err := factory.organizationSchemaReader.OrganizationSchemaOfOrganization(
		ctx, factory.NewExecutor(), organizationId)
	if err != nil {
		return models.DatabaseSchema{}, err
	}

	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   organizationSchema.DatabaseSchema.Database,
		Schema:     organizationSchema.DatabaseSchema.Schema,
	}, nil
}

func (factory DbExecutorFactory) TransactionInOrgSchema(
	ctx context.Context,
	organizationId string,
	f func(tx repositories.Transaction) error,
) error {
	dbSchema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return err
	}

	return factory.transactionFactoryRepository.Transaction(ctx, dbSchema, f)
}

func (factory DbExecutorFactory) Transaction(
	ctx context.Context,
	f func(tx repositories.Transaction) error,
) error {
	return factory.transactionFactoryRepository.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, f)
}

func (factory DbExecutorFactory) NewClientDbExecutor(
	ctx context.Context,
	organizationId string,
) (repositories.Executor, error) {
	schema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return repositories.PgExecutor{}, err
	}

	return factory.transactionFactoryRepository.GetExecutor(schema), nil
}

func (factory DbExecutorFactory) NewExecutor() repositories.Executor {
	return factory.transactionFactoryRepository.GetExecutor(models.DATABASE_MARBLE_SCHEMA)
}
