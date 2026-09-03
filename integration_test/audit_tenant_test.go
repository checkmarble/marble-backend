package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrantAuditIncludesTenant(t *testing.T) {
	organizationID := uuid.New()
	tenantID := uuid.New()
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, models.Credentials{
		ActorIdentity:  models.Identity{UserId: "audit-test-user"},
		OrganizationId: organizationID,
		TenantId:       tenantID,
	})
	grantID := uuid.New()

	_, err := pgPool.Exec(context.Background(), "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(context.Background(),
		"INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
		organizationID, organizationID.String(), tenantID)
	require.NoError(t, err)

	err = testUsecases.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
			VALUES ($1, 'user', 'audit-test-principal', 'marble', $2, 'TENANT_ADMIN')
		`, grantID, tenantID)
		return err
	})
	require.NoError(t, err)

	var orgID, auditedTenantID uuid.UUID
	var userID, tableName, operation string
	err = pgPool.QueryRow(context.Background(), `
		SELECT org_id, tenant_id, user_id, "table", operation
		FROM audit.audit_events
		WHERE entity_id = $1 AND "table" = 'grants'
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, grantID).Scan(&orgID, &auditedTenantID, &userID, &tableName, &operation)
	require.NoError(t, err)
	require.Equal(t, organizationID, orgID)
	require.Equal(t, tenantID, auditedTenantID)
	require.Equal(t, "audit-test-user", userID)
	require.Equal(t, "grants", tableName)
	require.Equal(t, "INSERT", operation)
}
