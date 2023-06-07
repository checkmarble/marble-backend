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
	CreateTable(tx Transaction, schema string, table models.Table) error
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

func (repo *ClientTablesRepositoryPostgresql) CreateTable(tx Transaction, schema string, table models.Table) error {
	pgTx := repo.toPostgresTransaction(tx)

	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, string(table.Name)})
	createTableExpr := squirrel.Expr(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", sanitizedTableName))

	idColumn := squirrel.Expr("id uuid DEFAULT uuid_generate_v4(),")
	createTableExpr = squirrel.ConcatExpr(createTableExpr, idColumn)

	validFromColumn := squirrel.Expr("valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),")
	createTableExpr = squirrel.ConcatExpr(createTableExpr, validFromColumn)

	validUntilColumn := squirrel.Expr("valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY',")
	createTableExpr = squirrel.ConcatExpr(createTableExpr, validUntilColumn)

	for fieldName, field := range table.Fields {
		columnExpr := fmt.Sprintf("%s %s", pgx.Identifier.Sanitize([]string{string(fieldName)}), toPgType(field.DataType))
		if !field.Nullable {
			columnExpr = fmt.Sprintf("%s NOT NULL", columnExpr)
		}
		createTableExpr = squirrel.ConcatExpr(createTableExpr, columnExpr, ",")
	}

	createTableExpr = squirrel.ConcatExpr(createTableExpr, "PRIMARY KEY(id));")

	createTableExpr = squirrel.ConcatExpr(createTableExpr, fmt.Sprintf("CREATE INDEX ON %s(object_id, valid_until DESC, valid_from, updated_at);", sanitizedTableName))

	sql, args, err := createTableExpr.ToSql()
	if err != nil {
		return err
	}

	sql, err = squirrel.Dollar.ReplacePlaceholders(sql)
	if err != nil {
		return err
	}

	_, err = pgTx.Exec(sql, args...)
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

func toPgType(dataType models.DataType) string {
	switch dataType {
	case models.String:
		return "TEXT"
	case models.Timestamp:
		return "TIMESTAMP WITH TIME ZONE"
	case models.Float:
		return "FLOAT"
	case models.Bool:
		return "BOOLEAN"
	default:
		panic(fmt.Errorf("unknown data type: %v", dataType))
	}
}
