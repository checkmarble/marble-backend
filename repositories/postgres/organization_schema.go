package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func (tx *Transaction) OrganizationSchemaOfOrganization(ctx context.Context, organizationID string) (string, error) {
	query := `
		SELECT schema_name
		FROM organizations_schema
		WHERE org_id = $1
	`

	var schema string
	err := tx.QueryRow(ctx, query, organizationID).Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("tx.QueryRow error: %w", err)
	}
	return schema, nil
}

func (tx *Transaction) addTableToSchema(ctx context.Context, schema, name string) error {
	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, name})

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL DEFAULT uuid_generate_v4(),
			object_id TEXT NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT 'INFINITY'
		)
	`, sanitizedTableName)

	_, err := tx.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("tx.Exec error: %w", err)
	}
	return nil
}

func (tx *Transaction) addDataModelFieldToSchema(ctx context.Context, schema, tableName string, field models.DataModelField) error {
	fieldType := models.DataTypeFrom(field.Type).ToPostgresType()
	sanitizedTableName := pgx.Identifier.Sanitize([]string{schema, tableName})

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s",
		sanitizedTableName, field.Name, fieldType))
	if !field.Nullable {
		builder.WriteString(" NOT NULL")
	}
	_, err := tx.Exec(ctx, builder.String())
	return err
}

func (tx *Transaction) createSchema(ctx context.Context, schema string) error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s",
		pgx.Identifier.Sanitize([]string{schema}))

	_, err := tx.Exec(ctx, query)
	return err
}
