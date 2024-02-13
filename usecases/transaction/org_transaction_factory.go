package transaction

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type Factory_deprec interface {
	TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Transaction_deprec) error) error
}

type FactoryImpl_deprec struct {
	OrganizationSchemaRepository repositories.OrganizationSchemaRepository
	TransactionFactory           TransactionFactory_deprec
}

func (factory *FactoryImpl_deprec) organizationDatabaseSchema(ctx context.Context, organizationId string) (models.DatabaseSchema, error) {
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

func (factory *FactoryImpl_deprec) TransactionInOrgSchema(ctx context.Context, organizationId string, f func(tx repositories.Transaction_deprec) error) error {

	dbSchema, err := factory.organizationDatabaseSchema(ctx, organizationId)
	if err != nil {
		return err
	}

	return factory.TransactionFactory.Transaction(ctx, dbSchema, f)
}

// helper
func InOrganizationSchema_deprec[ReturnType any](
	ctx context.Context,
	factory Factory_deprec,
	organizationId string,
	fn func(tx repositories.Transaction_deprec) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(ctx, organizationId, func(tx repositories.Transaction_deprec) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
