package transaction

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type Factory interface {
	TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Transaction) error) error
}

type FactoryImpl struct {
	OrganizationSchemaRepository repositories.OrganizationSchemaRepository
	TransactionFactory           TransactionFactory
}

func (factory *FactoryImpl) organizationDatabaseSchema(ctx context.Context, organizationId string) (models.DatabaseSchema, error) {
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

	dbSchema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return err
	}

	return factory.TransactionFactory.Transaction(ctx, dbSchema, f)
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

func transactionReturnValue[ReturnType any](
	ctx context.Context,
	factory TransactionFactory,
	databaseSchema models.DatabaseSchema,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.Transaction(ctx, databaseSchema, func(tx repositories.Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
