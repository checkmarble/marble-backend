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

func TestDeleteRoleOnlyWhenUnused(t *testing.T) {
	orgId := uuid.New()
	tenantId := uuid.New()
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, models.Credentials{
		ActorIdentity:  models.Identity{UserId: "delete-role-test-user"},
		OrganizationId: orgId,
		TenantId:       tenantId,
	})

	_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantId, tenantId.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
		orgId, orgId.String(), tenantId)
	require.NoError(t, err)

	repository := testUsecases.Repositories.MarbleDbRepository
	transactionFactory := testUsecases.NewTransactionFactory()

	role, err := repository.CreateRole(ctx, testUsecases.NewExecutorFactory().NewExecutor(), orgId, "org/deletable", "Deletable")
	require.NoError(t, err)

	insertGrant := func(principalType, principalId, revokedAt, expiresAt string) uuid.UUID {
		grantId := uuid.New()

		_, err := pgPool.Exec(ctx, `
			INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role, custom_role_id, revoked_at, expires_at)
			VALUES ($1, $2, $3, 'marble', $4, $5, $6, `+revokedAt+`, `+expiresAt+`)
		`, grantId, principalType, principalId, orgId, role.Slug, role.Id)
		require.NoError(t, err)

		return grantId
	}

	deleteRole := func() error {
		return transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			return repository.DeleteRole(ctx, tx, orgId, role.Id)
		})
	}

	revokedGrant := insertGrant("user", uuid.NewString(), "now()", "null")
	expiredGrant := insertGrant("user", uuid.NewString(), "null", "now() - interval '1 day'")
	userGrant := insertGrant("user", uuid.NewString(), "null", "null")
	apiKeyGrant := insertGrant("api_key", uuid.NewString(), "null", "now() + interval '1 day'")

	groupId := uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO groups (id, organization_id, name) VALUES ($1, $2, 'Reviewers')", groupId, orgId)
	require.NoError(t, err)

	groupGrant := insertGrant("group", groupId.String(), "null", "null")

	require.True(t, repositories.IsForeignKeyViolationError(deleteRole()))

	_, err = pgPool.Exec(ctx, "UPDATE grants SET revoked_at = now() WHERE id = $1", userGrant)
	require.NoError(t, err)

	require.True(t, repositories.IsForeignKeyViolationError(deleteRole()))

	_, err = pgPool.Exec(ctx, "UPDATE grants SET revoked_at = now() WHERE id = $1", apiKeyGrant)
	require.NoError(t, err)

	require.True(t, repositories.IsForeignKeyViolationError(deleteRole()))

	err = transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return repository.DeleteGroup(ctx, tx, orgId, groupId)
	})
	require.NoError(t, err)

	require.NoError(t, deleteRole())
	require.ErrorIs(t, deleteRole(), models.NotFoundError)

	var remaining int
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM grants WHERE id = ANY($1)",
		[]uuid.UUID{revokedGrant, expiredGrant, userGrant, apiKeyGrant, groupGrant}).Scan(&remaining)
	require.NoError(t, err)
	require.Zero(t, remaining)
}
