package integration

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAutoAssignmentFollowsOrganizationGrants(t *testing.T) {
	ctx := t.Context()
	tenantID, orgID, homeOrgID := uuid.New(), uuid.New(), uuid.New()
	inboxID, otherInboxID, homeInboxID := uuid.New(), uuid.New(), uuid.New()
	userID, caseID := uuid.New(), uuid.New()
	_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	for _, id := range []uuid.UUID{orgID, homeOrgID} {
		_, err = pgPool.Exec(ctx, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)", id, id.String(), tenantID)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pgPool.Exec(bg, "DELETE FROM cases WHERE org_id = ANY($1)", []uuid.UUID{orgID, homeOrgID})
		_, _ = pgPool.Exec(bg, "DELETE FROM inbox_users WHERE user_id = $1", userID)
		_, _ = pgPool.Exec(bg, "DELETE FROM grants WHERE principal_type = 'user' AND principal_id = $1", userID.String())
		_, _ = pgPool.Exec(bg, "DELETE FROM users WHERE id = $1", userID)
		_, _ = pgPool.Exec(bg, "DELETE FROM inboxes WHERE id = ANY($1)", []uuid.UUID{inboxID, otherInboxID, homeInboxID})
		_, _ = pgPool.Exec(bg, "DELETE FROM organizations WHERE id = ANY($1)", []uuid.UUID{orgID, homeOrgID})
		_, _ = pgPool.Exec(bg, "DELETE FROM tenants WHERE id = $1", tenantID)
	})
	for _, inbox := range []struct{ id, org uuid.UUID }{{inboxID, orgID}, {otherInboxID, orgID}, {homeInboxID, homeOrgID}} {
		_, err = pgPool.Exec(ctx, "INSERT INTO inboxes (id, organization_id, name, auto_assign_enabled) VALUES ($1, $2, 'inbox', true)", inbox.id, inbox.org)
		require.NoError(t, err)
	}
	_, err = pgPool.Exec(ctx, "INSERT INTO users (id, email, role, organization_id) VALUES ($1, $2, $3, $4)", userID, userID.String()+"@example.test", models.LegacyRoleValue(models.NativeRoleBindings([]models.Role{models.VIEWER})), homeOrgID)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{inboxID, otherInboxID, homeInboxID} {
		_, err = pgPool.Exec(ctx, "INSERT INTO inbox_users (inbox_id, user_id, auto_assignable) VALUES ($1, $2, true)", id, userID)
		require.NoError(t, err)
	}
	_, err = pgPool.Exec(ctx, "INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role) VALUES ($1, 'user', $2, 'marble', $3, 'VIEWER'), ($4, 'user', $2, 'marble', $3, 'BUILDER')", uuid.New(), userID.String(), orgID, uuid.New())
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO cases (id, org_id, inbox_id, name) VALUES ($1, $2, $3, 'pending')", caseID, orgID, inboxID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO cases (org_id, inbox_id, name, assigned_to) VALUES ($1, $2, 'home workload', $3)", homeOrgID, homeInboxID, userID)
	require.NoError(t, err)

	repo := repositories.NewMarbleDbRepository(false, 0, auditTestKeeper{})
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	assertEligible := func(want bool) {
		t.Helper()
		users, err := repo.FindAutoAssignableUsers(ctx, exec, orgID, 1)
		require.NoError(t, err)
		cases, err := repo.FindAutoAssignableCases(ctx, exec, orgID, 1)
		require.NoError(t, err)
		user, err := repo.FindNextAutoAssignableUserForInbox(ctx, exec, orgID, inboxID, 1)
		require.NoError(t, err)
		if want {
			require.Len(t, users, 1)
			require.Equal(t, 0, users[0].CaseCount)
			require.Len(t, cases, 1)
			require.Equal(t, caseID.String(), cases[0].Id)
			require.NotNil(t, user)
			require.Equal(t, models.UserId(userID.String()), user.UserId)
		} else {
			require.Empty(t, users)
			require.Empty(t, cases)
			require.Nil(t, user)
		}
	}
	assertEligible(true)
	uc := testUsecases.NewAutoAssignmentUsecase()
	require.NoError(t, uc.RunAutoAssigner(ctx, orgID, inboxID))
	var assignedTo uuid.UUID
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT assigned_to FROM cases WHERE id = $1", caseID).Scan(&assignedTo))
	require.Equal(t, userID, assignedTo)
	_, err = pgPool.Exec(ctx, "UPDATE cases SET assigned_to = NULL WHERE id = $1", caseID)
	require.NoError(t, err)
	user, err := repo.FindNextAutoAssignableUserForInbox(ctx, exec, orgID, homeInboxID, 1)
	require.NoError(t, err)
	require.Nil(t, user)

	_, err = pgPool.Exec(ctx, "UPDATE grants SET revoked_at = now() WHERE organization_id = $1 AND principal_id = $2", orgID, userID.String())
	require.NoError(t, err)
	assertEligible(false)
	_, err = pgPool.Exec(ctx, "UPDATE grants SET revoked_at = NULL, expires_at = now() - interval '1 hour' WHERE organization_id = $1 AND principal_id = $2", orgID, userID.String())
	require.NoError(t, err)
	assertEligible(false)
	_, err = pgPool.Exec(ctx, "UPDATE grants SET expires_at = NULL WHERE organization_id = $1 AND principal_id = $2", orgID, userID.String())
	require.NoError(t, err)
	assertEligible(true)
	_, err = pgPool.Exec(ctx, "UPDATE grants SET revoked_at = now() WHERE organization_id = $1 AND principal_id = $2 AND role = 'VIEWER'", orgID, userID.String())
	require.NoError(t, err)
	assertEligible(true)
	_, err = pgPool.Exec(ctx, "INSERT INTO user_unavailabilities (org_id, user_id, from_date, until_date) VALUES ($1, $2, now() - interval '1 hour', now() + interval '1 hour')", orgID, userID)
	require.NoError(t, err)
	assertEligible(false)
	_, err = pgPool.Exec(ctx, "DELETE FROM user_unavailabilities WHERE org_id = $1 AND user_id = $2", orgID, userID)
	require.NoError(t, err)
	assertEligible(true)
	_, err = pgPool.Exec(ctx, "INSERT INTO cases (org_id, inbox_id, name, assigned_to) VALUES ($1, $2, 'org workload', $3)", orgID, inboxID, userID)
	require.NoError(t, err)
	assertEligible(false)
}
