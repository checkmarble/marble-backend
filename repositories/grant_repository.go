package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type GrantRepository interface {
	EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error
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
