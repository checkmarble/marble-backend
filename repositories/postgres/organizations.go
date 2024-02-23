package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func (db *Database) GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error) {
	query := `
		SELECT id, name, export_scheduled_execution_s3
		FROM organizations
		WHERE id = $1
	`

	var organization models.Organization
	err := db.pool.QueryRow(ctx, query, organizationID).Scan(
		&organization.Id,
		&organization.Name,
		// &organization.DatabaseName,
		&organization.ExportScheduledExecutionS3,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Organization{}, models.NotFoundError
	}
	if err != nil {
		return models.Organization{}, fmt.Errorf("pool.QueryRow error: %w", err)
	}
	return organization, nil
}
