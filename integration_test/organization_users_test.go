package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUsersOfOrganizationFollowsGrants(t *testing.T) {
	ctx := t.Context()
	tenantID := uuid.New()
	organizationID := uuid.New()
	otherOrganizationID := uuid.New()

	_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	for _, id := range []uuid.UUID{organizationID, otherOrganizationID} {
		_, err = pgPool.Exec(ctx, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
			id, id.String(), tenantID)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pgPool.Exec(bg, "DELETE FROM grants WHERE organization_id = ANY($1) OR tenant_id = $2",
			[]uuid.UUID{organizationID, otherOrganizationID}, tenantID)
		_, _ = pgPool.Exec(bg, "DELETE FROM users WHERE organization_id = ANY($1)",
			[]uuid.UUID{organizationID, otherOrganizationID})
		_, _ = pgPool.Exec(bg, "DELETE FROM organizations WHERE id = ANY($1)",
			[]uuid.UUID{organizationID, otherOrganizationID})
		_, _ = pgPool.Exec(bg, "DELETE FROM tenants WHERE id = $1", tenantID)
	})

	createUser := func(homeOrganizationID uuid.UUID, homeRole models.Role) string {
		t.Helper()
		userID := uuid.NewString()
		_, err := pgPool.Exec(ctx, "INSERT INTO users (id, email, role, organization_id) VALUES ($1, $2, $3, $4)",
			userID, "org-users-"+userID+"@example.test", int(homeRole), homeOrganizationID)
		require.NoError(t, err)
		return userID
	}
	grantOrganization := func(userID string, role models.Role, extra string) {
		t.Helper()
		_, err := pgPool.Exec(ctx, `INSERT INTO grants
			(id, principal_type, principal_id, principal_authority, organization_id, role, revoked_at, expires_at)
			VALUES ($1, 'user', $2, 'marble', $3, $4,
				CASE WHEN $5 = 'revoked' THEN NOW() END,
				CASE WHEN $5 = 'expired' THEN NOW() - INTERVAL '1 day' END)`,
			uuid.New(), userID, organizationID, role.String(), extra)
		require.NoError(t, err)
	}

	homeUser := createUser(organizationID, models.ADMIN)
	grantOrganization(homeUser, models.BUILDER, "")

	guest := createUser(otherOrganizationID, models.VIEWER)
	grantOrganization(guest, models.PUBLISHER, "")

	homeWithoutGrant := createUser(organizationID, models.ADMIN)

	revoked := createUser(organizationID, models.VIEWER)
	grantOrganization(revoked, models.VIEWER, "revoked")

	expired := createUser(organizationID, models.VIEWER)
	grantOrganization(expired, models.VIEWER, "expired")

	deleted := createUser(organizationID, models.VIEWER)
	grantOrganization(deleted, models.VIEWER, "")
	_, err = pgPool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", deleted)
	require.NoError(t, err)

	tenantOnly := createUser(otherOrganizationID, models.VIEWER)
	_, err = pgPool.Exec(ctx, `INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
		VALUES ($1, 'user', $2, 'marble', $3, 'TENANT_ADMIN')`, uuid.New(), tenantOnly, tenantID)
	require.NoError(t, err)

	repo := repositories.NewMarbleDbRepository(false, 0, auditTestKeeper{})
	users, err := repo.ListUsers(ctx, testUsecases.NewExecutorFactory().NewExecutor(), &organizationID)
	require.NoError(t, err)

	rolesByUser := make(map[string]models.Role, len(users))
	for _, user := range users {
		assert.Equal(t, organizationID, user.OrganizationId, "user %s should be seen from the listed organization", user.UserId)
		rolesByUser[string(user.UserId)] = user.Role
	}

	assert.Equal(t, map[string]models.Role{
		homeUser: models.BUILDER,
		guest:    models.PUBLISHER,
	}, rolesByUser)
	for _, excluded := range []string{homeWithoutGrant, revoked, expired, deleted, tenantOnly} {
		assert.NotContains(t, rolesByUser, excluded)
	}
}
