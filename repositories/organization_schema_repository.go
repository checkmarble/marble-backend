package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
)

type OrganizationSchemaRepository interface {
	CreateSchemaIfNotExists(ctx context.Context, exec Executor) error
	DeleteSchema(ctx context.Context, exec Executor) error
	CreateTable(ctx context.Context, exec Executor, tableName string) error
	CreateField(ctx context.Context, exec Executor, tableName string, field models.CreateFieldInput) error
	RenameField(ctx context.Context, exec Executor, tableName string, fieldName string) error
	DeleteField(ctx context.Context, exec Executor, tableName string, fieldName string) error
}

type OrganizationSchemaRepositoryPostgresql struct{}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateSchemaIfNotExists(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s",
		pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema}))

	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) DeleteSchema(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE",
		pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema}))
	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateTable(ctx context.Context, exec Executor, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sanitizedTableName := pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id UUID NOT NULL PRIMARY KEY,
		object_id TEXT NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY'
	  )`, sanitizedTableName)

	_, err := exec.Exec(ctx, sql)
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) CreateField(
	ctx context.Context,
	exec Executor,
	tableName string,
	field models.CreateFieldInput,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	fieldType := toPgType(field.DataType)
	sanitizedTableName := pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s",
		sanitizedTableName, field.Name, fieldType))

	_, err := exec.Exec(ctx, builder.String())
	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) RenameField(
	ctx context.Context,
	exec Executor,
	tableName string,
	fieldName string,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	nonce := utils.GenNonce(8)
	padding := len(nonce) + len("old_") + 1
	trimmedName := fieldName[:min(len(fieldName), 63-padding)]

	sanitizedTableName := pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
	sanitizeNewFieldName := pgx.Identifier.Sanitize([]string{fmt.Sprintf("old_%s_%s", trimmedName, nonce)})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		sanitizedTableName, fieldName, sanitizeNewFieldName))

	_, err := exec.Exec(ctx, builder.String())
	if err != nil {
		return err
	}

	builder = strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL",
		sanitizedTableName, sanitizeNewFieldName))

	_, err = exec.Exec(ctx, builder.String())

	return err
}

func (repo *OrganizationSchemaRepositoryPostgresql) DeleteField(
	ctx context.Context,
	exec Executor,
	tableName string,
	fieldName string,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sanitizedTableName := pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s",
		sanitizedTableName, fieldName))

	_, err := exec.Exec(ctx, builder.String())
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
