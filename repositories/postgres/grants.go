package postgres

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

func (db *Database) ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.Grant, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT g.role, g.tenant_id, g.organization_id
		FROM active_grants g
		WHERE g.principal_type = $1 AND g.principal_id = $2 AND g.principal_authority = 'marble'
	`, principalType, principalID)
	if err != nil {
		return nil, fmt.Errorf("querying active grants: %w", err)
	}
	defer rows.Close()

	grants := []models.Grant{}
	for rows.Next() {
		var grant models.Grant
		var role string
		var tenantID, organizationID *uuid.UUID
		if err := rows.Scan(&role, &tenantID, &organizationID); err != nil {
			return nil, fmt.Errorf("scanning active grant: %w", err)
		}
		grant.Role = models.RoleFromString(role)
		if tenantID != nil {
			grant.TenantId = *tenantID
		}
		if organizationID != nil {
			grant.OrganizationId = *organizationID
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating active grants: %w", err)
	}
	return grants, nil
}
