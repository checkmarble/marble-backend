package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

// interfaces used by the class
type executorFactoryRepository interface {
	GetExecutor(ctx context.Context, typ models.DatabaseSchemaType, org *models.Organization) (repositories.Executor, error)
	GetPinnedExecutor(ctx context.Context, typ models.DatabaseSchemaType, org *models.Organization) (repositories.Executor, func(), error)
	// GetExecutor(ctx context.Context, typ models.DatabaseSchemaType, org *models.Organization, orgId uuid.UUID) (repositories.Executor, error)
	Transaction(
		ctx context.Context,
		typ models.DatabaseSchemaType,
		org *models.Organization,
		fn func(tx repositories.Transaction) error,
	) error
}

type organizationGetter interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
}

type DbExecutorFactory struct {
	appName                      string
	orgGetter                    organizationGetter
	transactionFactoryRepository executorFactoryRepository
	orgId                        uuid.UUID
}

func NewDbExecutorFactory(
	appName string,
	orgGetter organizationGetter,
	transactionFactoryRepository executorFactoryRepository,
	orgId uuid.UUID,
) DbExecutorFactory {
	return DbExecutorFactory{
		appName:                      appName,
		orgGetter:                    orgGetter,
		transactionFactoryRepository: transactionFactoryRepository,
		orgId:                        orgId,
	}
}

func (factory DbExecutorFactory) TransactionInOrgSchema(
	ctx context.Context,
	organizationId uuid.UUID,
	f func(tx repositories.Transaction) error,
) error {
	org, err := factory.orgGetter.GetOrganizationById(ctx, factory.NewExecutor(), organizationId)
	if err != nil {
		return err
	}

	return factory.transactionFactoryRepository.Transaction(ctx,
		models.DATABASE_SCHEMA_TYPE_CLIENT, &org, f)
}

func (factory DbExecutorFactory) Transaction(
	ctx context.Context,
	f func(tx repositories.Transaction) error,
) error {
	// for a DATABASE_SCHEMA_TYPE_MARBLE type transaction, we don't need to pass the organization because it just
	// uses the existing pool and default schema
	return factory.transactionFactoryRepository.Transaction(
		ctx,
		models.DATABASE_SCHEMA_TYPE_MARBLE, nil,
		f)
}

func (factory DbExecutorFactory) NewClientDbExecutor(
	ctx context.Context,
	organizationId uuid.UUID,
) (repositories.Executor, error) {
	org, err := factory.orgGetter.GetOrganizationById(ctx, factory.NewExecutor(), organizationId)
	if err != nil {
		return nil, err
	}

	return factory.transactionFactoryRepository.GetExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_CLIENT,
		&org,
	)
}

func (factory DbExecutorFactory) NewPinnedExecutor(
	ctx context.Context,
) (repositories.Executor, func(), error) {
	return factory.transactionFactoryRepository.GetPinnedExecutor(
		ctx,
		models.DATABASE_SCHEMA_TYPE_MARBLE,
		nil,
	)
}

func (factory DbExecutorFactory) NewExecutor() repositories.Executor {
	// when getting a marble db executor, no error should occur and the context also won't be used
	exec, _ := factory.transactionFactoryRepository.GetExecutor(
		context.Background(),
		models.DATABASE_SCHEMA_TYPE_MARBLE,
		nil)
	return exec
}
