package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// interfaces used by the class
type executorFactoryRepository interface {
	GetExecutor(ctx context.Context, typ models.DatabaseSchemaType, organizationId string,
		organizationName string) (repositories.Executor, error)
	Transaction(
		ctx context.Context,
		typ models.DatabaseSchemaType,
		organizationId string,
		organizationName string,
		fn func(tx repositories.Transaction) error,
	) error
}

type organizationGetter interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type DbExecutorFactory struct {
	orgGetter                    organizationGetter
	transactionFactoryRepository executorFactoryRepository
}

func NewDbExecutorFactory(
	orgGetter organizationGetter,
	transactionFactoryRepository executorFactoryRepository,
) DbExecutorFactory {
	return DbExecutorFactory{
		orgGetter:                    orgGetter,
		transactionFactoryRepository: transactionFactoryRepository,
	}
}

// func (factory DbExecutorFactory) organizationDatabaseSchema(
// 	ctx context.Context,
// 	organizationId string,
// ) (models.DatabaseSchema, error) {
// 	organizationSchema, err := factory.orgGetter.OrganizationSchemaOfOrganization(
// 		ctx, factory.NewExecutor(), organizationId)
// 	if err != nil {
// 		return models.DatabaseSchema{}, err
// 	}

// 	return models.DatabaseSchema{
// 		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
// 		Schema:     organizationSchema.DatabaseSchema.Schema,
// 	}, nil
// }

func (factory DbExecutorFactory) TransactionInOrgSchema(
	ctx context.Context,
	organizationId string,
	f func(tx repositories.Transaction) error,
) error {
	org, err := factory.orgGetter.GetOrganizationById(ctx, factory.NewExecutor(), organizationId)
	if err != nil {
		return err
	}

	return factory.transactionFactoryRepository.Transaction(ctx,
		models.DATABASE_SCHEMA_TYPE_CLIENT, organizationId, org.Name, f)
}

func (factory DbExecutorFactory) Transaction(
	ctx context.Context,
	f func(tx repositories.Transaction) error,
) error {
	// for a DATABASE_SCHEMA_TYPE_MARBLE type transaction, we don't need to pass organizationId and organizationName because it just
	// uses the existing pool and default schema
	return factory.transactionFactoryRepository.Transaction(
		ctx,
		models.DATABASE_SCHEMA_TYPE_MARBLE, "", "",
		f)
}

func (factory DbExecutorFactory) NewClientDbExecutor(
	ctx context.Context,
	organizationId string,
) (repositories.Executor, error) {
	org, err := factory.orgGetter.GetOrganizationById(ctx, factory.NewExecutor(), organizationId)
	if err != nil {
		return nil, err
	}

	return factory.transactionFactoryRepository.GetExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_CLIENT,
		organizationId,
		org.Name,
	)
}

func (factory DbExecutorFactory) NewExecutor() repositories.Executor {
	// when getting a marble db executor, no error should occur and the context also won't be used
	exec, _ := factory.transactionFactoryRepository.GetExecutor(
		context.Background(),
		models.DATABASE_SCHEMA_TYPE_MARBLE,
		"", "")
	return exec
}
