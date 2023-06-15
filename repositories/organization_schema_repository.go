package repositories

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrganizationSchemaRepository interface {
	OrganizationSchemaOfOrganization(tx Transaction, organizationId string) (models.OrganizationSchema, error)
	CreateOrganizationSchema(tx Transaction, createOrganizationSchema models.OrganizationSchema) error
	CreateSchema(tx Transaction, schema string) error
	DeleteSchema(tx Transaction, schema string) error
	CreateTable(tx Transaction, schema string, table models.Table) error
}

type OrganizationSchemaRepositoryPostgresql struct {
	transactionFactory TransactionFactory
	queryBuilder       squirrel.StatementBuilderType
}

func (repo *OrganizationSchemaRepositoryPostgresql) OrganizationSchemaOfOrganization(tx Transaction, organizationId string) (models.OrganizationSchema, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		repo.queryBuilder.
			Select(dbmodels.OrganizationSchemaFields...).
			From(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptOrganizationSchema,
	)

}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateSchema(tx Transaction, schema string) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sql := fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier.Sanitize([]string{schema}))

	_, err := pgTx.SqlExec(sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) DeleteSchema(tx Transaction, schema string) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sql := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pgx.Identifier.Sanitize([]string{schema}))

	_, err := pgTx.SqlExec(sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateTable(tx Transaction, schema string, table models.Table) error {
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

	_, err = pgTx.SqlExec(sql, args...)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateOrganizationSchema(tx Transaction, createOrganizationSchema models.OrganizationSchema) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		repo.queryBuilder.Insert(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Columns(
				dbmodels.OrganizationSchemaFields...,
			).
			Values(
				uuid.NewString(),
				createOrganizationSchema.OrganizationId,
				createOrganizationSchema.DatabaseSchema.Schema,
			),
	)
	return err
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
