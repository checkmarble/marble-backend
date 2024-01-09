package transaction

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Factory interface {
	OrganizationDatabaseSchema(ctx context.Context, organizationId string) (models.DatabaseSchema, error)
	TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Transaction) error) error
	// used by legacy code that didn't support transactions
	OrganizationDbPool(dbSchema models.DatabaseSchema) (*pgxpool.Pool, error)
}

type FactoryImpl struct {
	OrganizationSchemaRepository     repositories.OrganizationSchemaRepository
	TransactionFactory               TransactionFactory
	DatabaseConnectionPoolRepository repositories.DatabaseConnectionPoolRepository
}

func (factory *FactoryImpl) OrganizationDatabaseSchema(ctx context.Context, organizationId string) (models.DatabaseSchema, error) {
	organizationSchema, err := factory.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, nil, organizationId)
	if err != nil {
		return models.DatabaseSchema{}, err
	}

	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   organizationSchema.DatabaseSchema.Database,
		Schema:     organizationSchema.DatabaseSchema.Schema,
	}, nil
}

func (factory *FactoryImpl) TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Transaction) error) error {

	dbSchema, err := factory.OrganizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return err
	}

	return factory.TransactionFactory.Transaction(ctx, dbSchema, f)
}

func (factory *FactoryImpl) OrganizationDbPool(dbSchema models.DatabaseSchema) (*pgxpool.Pool, error) {

	return factory.DatabaseConnectionPoolRepository.DatabaseConnectionPool(dbSchema.Database.Connection)
}

// helper
func InOrganizationSchema[ReturnType any](
	ctx context.Context,
	factory Factory,
	organizationId string,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
