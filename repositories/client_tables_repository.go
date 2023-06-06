package repositories

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ClientTablesRepository interface {
	CreateClientTables(tx Transaction, createClientTable models.ClientTables) error
	CreateSchema(tx Transaction, schema string) error
}

type ClientTablesRepositoryPostgresql struct {
	queryBuilder squirrel.StatementBuilderType
}

func (repo *ClientTablesRepositoryPostgresql) CreateSchema(tx Transaction, schema string) error {
	pgTx := repo.toPostgresTransaction(tx)

	sql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgx.Identifier.Sanitize([]string{schema}))

	_, err := pgTx.Exec(sql)
	return err
}

func (repo *ClientTablesRepositoryPostgresql) CreateClientTables(tx Transaction, createClientTable models.ClientTables) error {
	pgTx := repo.toPostgresTransaction(tx)

	return SqlInsert(
		pgTx,
		repo.queryBuilder.Insert(dbmodels.TABLE_CLIENT_TABLES).
			Columns(
				dbmodels.ClientTablesFields...,
			).
			Values(
				uuid.NewString(),
				createClientTable.OrganizationId,
				createClientTable.Schema,
			),
	)
}

func (repo *ClientTablesRepositoryPostgresql) toPostgresTransaction(transaction Transaction) TransactionPostgres {

	tx := transaction.(TransactionPostgres)
	if transaction.Database() != models.DATABASE_MARBLE {
		panic("UserRepositoryPostgresql can only handle transactions in DATABASE_MARBLE")
	}
	return tx
}
