package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrganizationSchemaRepository interface {
	OrganizationSchemaOfOrganization(ctx context.Context, exec Executor, organizationId string) (models.OrganizationSchema, error)
	CreateOrganizationSchema(
		ctx context.Context,
		exec Executor,
		organizationId, schemaName string,
	) error
	CreateSchema(ctx context.Context, exec Executor, schema string) error
	DeleteSchema(ctx context.Context, exec Executor, schema string) error
	CreateTable(ctx context.Context, exec Executor, schema, tableName string) error
	CreateField(ctx context.Context, exec Executor, schema, tableName string, field models.DataModelField) error
}

type OrganizationSchemaRepositoryPostgresql struct{}

func (repo *OrganizationSchemaRepositoryPostgresql) OrganizationSchemaOfOrganization(
	ctx context.Context, exec Executor, organizationId string,
) (models.OrganizationSchema, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.OrganizationSchema{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.OrganizationSchemaFields...).
			From(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptOrganizationSchema,
	)
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateSchema(ctx context.Context, exec Executor, schema string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s",
		pgx.Identifier.Sanitize([]string{schema}))

	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) DeleteSchema(ctx context.Context, exec Executor, schema string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE",
		pgx.Identifier.Sanitize([]string{schema}))
	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateTable(ctx context.Context, exec Executor, schema, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, tableName})
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id UUID NOT NULL DEFAULT uuid_generate_v4(),
		object_id TEXT NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY'
	  )`, sanitizedTableName)

	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateField(ctx context.Context, exec Executor,
	schema, tableName string, field models.DataModelField,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	fieldType := toPgType(models.DataTypeFrom(field.Type))
	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, tableName})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s",
		sanitizedTableName, field.Name, fieldType))
	if !field.Nullable {
		builder.WriteString(" NOT NULL")
	}
	_, err := exec.Exec(ctx, builder.String())
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateOrganizationSchema(
	ctx context.Context,
	exec Executor,
	organizationId, schemaName string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.ORGANIZATION_SCHEMA_TABLE).
			Columns(
				dbmodels.OrganizationSchemaFields...,
			).
			Values(
				uuid.NewString(),
				organizationId,
				schemaName,
			),
	)
	return err
}

func toPgType(dataType models.DataType) string {
	switch dataType {
	case models.Int:
		return "INTEGER"
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
