package organization

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrgTransactionFactory interface {
	OrganizationDatabaseSchema(organizationId string) (models.DatabaseSchema, error)
	TransactionInOrgSchema(organizationId string, f func(tx repositories.Transaction) error) error
	// used by legacy code that to not support transactions
	OrganizationPool(organizationId string) (*pgxpool.Pool, error)
}

type OrgTransactionFactoryImpl struct {
	ClientTablesRepository           repositories.ClientTablesRepository
	TransactionFactory               repositories.TransactionFactory
	databaseConnectionPoolRepository repositories.DatabaseConnectionPoolRepository
}

func (factory *OrgTransactionFactoryImpl) OrganizationDatabaseSchema(organizationId string) (models.DatabaseSchema, error) {
	clientTables, err := factory.ClientTablesRepository.ClientTableOfOrganization(nil, organizationId)
	if err != nil {
		return models.DatabaseSchema{}, err
	}

	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Database:   clientTables.DatabaseSchema.Database,
		Schema:     clientTables.DatabaseSchema.Schema,
	}, nil
}

func (factory *OrgTransactionFactoryImpl) TransactionInOrgSchema(organizationId string, f func(tx repositories.Transaction) error) error {

	dbSchema, err := factory.OrganizationDatabaseSchema(organizationId)
	if err != nil {
		return err
	}

	return factory.TransactionFactory.Transaction(dbSchema, f)
}

func (factory *OrgTransactionFactoryImpl) OrganizationPool(organizationId string) (*pgxpool.Pool, error) {

	dbSchema, err := factory.OrganizationDatabaseSchema(organizationId)
	if err != nil {
		return nil, err
	}

	return factory.databaseConnectionPoolRepository.DatabaseConnectionPool(dbSchema.Database.Connection)
}
