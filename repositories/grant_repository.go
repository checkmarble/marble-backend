package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type GrantRepository interface {
	EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error
	ListOrganizationsForUser(ctx context.Context, exec Executor, userID string) ([]models.Organization, error)
}

func (repo *MarbleDbRepository) EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	_, err := exec.Exec(ctx, `
		INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
		SELECT $1, 'user', $2, 'marble', tenant_id, $3 FROM organizations WHERE id = $4
		ON CONFLICT DO NOTHING
	`, pure_utils.NewId(), userID, models.TENANT_ADMIN.String(), organizationID)
	return err
}

func (repo *MarbleDbRepository) ListOrganizationsForUser(ctx context.Context, exec Executor, userID string) ([]models.Organization, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, `
		SELECT o.id, o.name, o.tenant_id, array_agg(DISTINCT scope_grants.role)
		FROM active_grants membership
		JOIN organizations o ON o.id = membership.organization_id
		JOIN active_grants scope_grants ON scope_grants.principal_type = membership.principal_type
			AND scope_grants.principal_id = membership.principal_id
			AND scope_grants.principal_authority = membership.principal_authority
			AND (scope_grants.organization_id = o.id OR scope_grants.tenant_id = o.tenant_id)
		WHERE membership.principal_type = 'user' AND membership.principal_id = $1 AND membership.principal_authority = 'marble'
		GROUP BY o.id, o.name, o.tenant_id
		ORDER BY o.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user organizations: %w", err)
	}
	defer rows.Close()
	organizations := []models.Organization{}
	for rows.Next() {
		var organization models.Organization
		var roles []string
		if err := rows.Scan(&organization.Id, &organization.Name, &organization.TenantId, &roles); err != nil {
			return nil, fmt.Errorf("scanning user organization: %w", err)
		}
		for _, role := range roles {
			organization.Roles = append(organization.Roles, models.RoleFromString(role))
		}
		organizations = append(organizations, organization)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user organizations: %w", err)
	}
	return organizations, nil
}
