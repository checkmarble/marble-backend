package organization

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrgTransactionFactory interface {
	OrganizationDatabaseSchema(organizationId string) (models.DatabaseSchema, error)
	TransactionInOrgSchema(organizationId string, f func(tx repositories.Transaction) error) error
	// used by legacy code that didn't support transactions
	OrganizationDbPool(dbSchema models.DatabaseSchema) (*pgxpool.Pool, error)
}

type OrgTransactionFactoryImpl struct {
	OrganizationSchemaRepository     repositories.OrganizationSchemaRepository
	TransactionFactory               repositories.TransactionFactory
	DatabaseConnectionPoolRepository repositories.DatabaseConnectionPoolRepository
}

func (factory *OrgTransactionFactoryImpl) OrganizationDatabaseSchema(organizationId string) (models.DatabaseSchema, error) {
	organizationSchema, err := factory.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(nil, organizationId)
	if err != nil {
		return models.DatabaseSchema{}, err
	}

	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   organizationSchema.DatabaseSchema.Database,
		Schema:     organizationSchema.DatabaseSchema.Schema,
	}, nil
}

func (factory *OrgTransactionFactoryImpl) TransactionInOrgSchema(organizationId string, f func(tx repositories.Transaction) error) error {

	dbSchema, err := factory.OrganizationDatabaseSchema(organizationId)
	if err != nil {
		return err
	}

	return factory.TransactionFactory.Transaction(dbSchema, f)
}

func (factory *OrgTransactionFactoryImpl) OrganizationDbPool(dbSchema models.DatabaseSchema) (*pgxpool.Pool, error) {

	return factory.DatabaseConnectionPoolRepository.DatabaseConnectionPool(dbSchema.Database.Connection)
}

// helper
func TransactionInOrgSchemaReturnValue[ReturnType any](
	factory OrgTransactionFactory,
	organizationId string,
	fn func(tx repositories.Transaction) (ReturnType, error),
) (ReturnType, error) {
	var value ReturnType
	transactionErr := factory.TransactionInOrgSchema(organizationId, func(tx repositories.Transaction) error {
		var fnErr error
		value, fnErr = fn(tx)
		return fnErr
	})
	return value, transactionErr
}
