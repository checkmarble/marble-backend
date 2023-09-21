package repositories

import (
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

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
	CreateSimpleTable(tx Transaction, schema, tableName string) error
	CreateSimpleField(tx Transaction, schema, tableName string, field models.DataModelField) error
}

type OrganizationSchemaRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *OrganizationSchemaRepositoryPostgresql) OrganizationSchemaOfOrganization(tx Transaction, organizationId string) (models.OrganizationSchema, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
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

func (repo *OrganizationSchemaRepositoryPostgresql) CreateSimpleTable(tx Transaction, schema, tableName string) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, tableName})
	createTableExpr := squirrel.Expr(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ()", sanitizedTableName))

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

func (repo *OrganizationSchemaRepositoryPostgresql) CreateSimpleField(tx Transaction, schema, tableName string, field models.DataModelField) error {
	pgTx := adaptClientDatabaseTransaction(tx)

	fieldType := toPgType(models.DataTypeFrom(field.Type))
	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, tableName})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", sanitizedTableName, field.Name, fieldType))
	if !field.Nullable {
		builder.WriteString(" NOT NULL")
	}
	_, err := pgTx.SqlExec(builder.String())
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
		NewQueryBuilder().Insert(dbmodels.ORGANIZATION_SCHEMA_TABLE).
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
