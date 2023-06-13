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
	ClientTableOfOrganization(tx Transaction, organizationId string) (models.ClientTables, error)
	CreateClientTables(tx Transaction, createClientTable models.ClientTables) error
	CreateSchema(tx Transaction, schema string) error
	DeleteSchema(tx Transaction, schema string) error
	CreateTable(tx Transaction, schema string, table models.Table) error
}

type ClientTablesRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *ClientTablesRepositoryPostgresql) ClientTableOfOrganization(tx Transaction, organizationId string) (models.ClientTables, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.ClientTablesFields...).
			From(dbmodels.TABLE_CLIENT_TABLES).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptClientTable,
	)

}

func (repo *ClientTablesRepositoryPostgresql) CreateSchema(tx Transaction, schema string) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sql := fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier.Sanitize([]string{schema}))

	_, err := pgTx.Exec(sql)
	return err
}

func (repo *ClientTablesRepositoryPostgresql) DeleteSchema(tx Transaction, schema string) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sql := fmt.Sprintf("DROP SCHEMA %s CASCADE", pgx.Identifier.Sanitize([]string{schema}))

	_, err := pgTx.Exec(sql)
	return err
}

func (repo *ClientTablesRepositoryPostgresql) CreateTable(tx Transaction, schema string, table models.Table) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, string(table.Name)})
	createTableExpr := squirrel.Expr(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", sanitizedTableName))

	idColumn := squirrel.Expr("id uuid,")
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
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlInsert(
		pgTx,
		repo.queryBuilder.Insert(dbmodels.TABLE_CLIENT_TABLES).
			Columns(
				dbmodels.ClientTablesFields...,
			).
			Values(
				uuid.NewString(),
				createClientTable.OrganizationId,
				createClientTable.DatabaseSchema.Schema,
			),
	)
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
