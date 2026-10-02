package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func (db *Database) GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (models.Organization, error) {
	query := `
		SELECT id, name, tenant_id, default_scenario_timezone
		FROM organizations
		WHERE id = $1
	`

	var organization models.Organization
	err := db.pool.QueryRow(ctx, query, organizationID).Scan(
		&organization.Id,
		&organization.Name,
		&organization.TenantId,
		&organization.DefaultScenarioTimezone,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Organization{}, models.NotFoundError
	}
	if err != nil {
		return models.Organization{}, fmt.Errorf("pool.QueryRow error: %w", err)
	}
	return organization, nil
}
