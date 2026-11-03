package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/utils"
	crdberrors "github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUserIdentity(t *testing.T) {
	ctx := utils.StoreLoggerInContext(t.Context(), utils.NewLogger("text"))
	database := postgres.New(pgPool)

	t.Run("links two issuers to the same user", func(t *testing.T) {
		userID := createIdentityTestUser(t)

		firebaseUser, err := database.ResolveUserIdentity(ctx, "firebase", "firebase-subject", identityTestEmail(userID))
		require.NoError(t, err)
		dexUser, err := database.ResolveUserIdentity(ctx, "dex", "dex-subject", identityTestEmail(userID))
		require.NoError(t, err)
		assert.Equal(t, userID, string(firebaseUser.UserId))
		assert.Equal(t, userID, string(dexUser.UserId))
		assert.Equal(t, 2, identityCount(t, userID))
	})

	t.Run("keeps the linked user when the identity email changes", func(t *testing.T) {
		originalUserID := createIdentityTestUser(t)
		otherUserID := createIdentityTestUser(t)

		_, err := database.ResolveUserIdentity(ctx, "dex", "subject", identityTestEmail(originalUserID))
		require.NoError(t, err)
		user, err := database.ResolveUserIdentity(ctx, "dex", "subject", identityTestEmail(otherUserID))
		require.NoError(t, err)
		assert.Equal(t, originalUserID, string(user.UserId))
		assert.Equal(t, 1, identityCount(t, originalUserID))
		assert.Equal(t, 0, identityCount(t, otherUserID))
	})

	t.Run("does not reassign an identity from a deleted user", func(t *testing.T) {
		deletedUserID := createIdentityTestUser(t)
		liveUserID := createIdentityTestUser(t)

		_, err := database.ResolveUserIdentity(ctx, "dex", "subject", identityTestEmail(deletedUserID))
		require.NoError(t, err)
		query, args, err := postgres.NewQueryBuilder().Update("users").
			Set("deleted_at", squirrel.Expr("NOW()")).
			Set("email", identityTestEmail(liveUserID)).
			Where(squirrel.Eq{"id": deletedUserID}).
			ToSql()
		require.NoError(t, err)
		_, err = pgPool.Exec(ctx, query, args...)
		require.NoError(t, err)

		_, err = database.ResolveUserIdentity(ctx, "dex", "subject", identityTestEmail(liveUserID))
		assert.ErrorIs(t, err, models.NotFoundError)
		assert.Equal(t, 1, identityCount(t, deletedUserID))
		assert.Equal(t, 0, identityCount(t, liveUserID))
	})

	t.Run("does not create an identity for an unknown email", func(t *testing.T) {
		_, err := database.ResolveUserIdentity(ctx, "dex", "subject", "unknown-"+uuid.NewString()+"@example.test")
		assert.ErrorIs(t, err, models.NotFoundError)

		var count int
		query, args, err := postgres.NewQueryBuilder().Select("COUNT(*)").
			From("user_identities").
			Where(squirrel.Eq{"issuer": "dex", "subject": "subject"}).
			ToSql()
		require.NoError(t, err)
		err = pgPool.QueryRow(ctx, query, args...).Scan(&count)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("tenant resolution cannot link an identity outside its tenant", func(t *testing.T) {
		tenantID := uuid.New()
		allowedUserID := createIdentityTestUser(t)
		outsideUserID := createIdentityTestUser(t)
		grantIdentityTestTenantAccess(t, tenantID, allowedUserID)

		_, err := database.ResolveUserIdentityForTenant(ctx, "tenant-dex", "outside-subject", identityTestEmail(outsideUserID), tenantID)
		assert.ErrorIs(t, err, models.NotFoundError)
		assert.Zero(t, identityCount(t, outsideUserID))

		user, err := database.ResolveUserIdentityForTenant(ctx, "tenant-dex", "allowed-subject", identityTestEmail(allowedUserID), tenantID)
		require.NoError(t, err)
		assert.Equal(t, allowedUserID, string(user.UserId))
		assert.Equal(t, 1, identityCount(t, allowedUserID))
	})

	t.Run("concurrent first logins create one identity", func(t *testing.T) {
		userID := createIdentityTestUser(t)
		const attempts = 8

		var wg sync.WaitGroup
		errors := make(chan error, attempts)
		for range attempts {
			wg.Go(func() {
				user, err := database.ResolveUserIdentity(ctx, "dex", "concurrent-subject", identityTestEmail(userID))
				if err == nil && string(user.UserId) != userID {

					err = crdberrors.Errorf("resolved user %s, want %s", user.UserId, userID)
				}
				errors <- err
			})
		}
		wg.Wait()
		close(errors)

		for err := range errors {
			assert.NoError(t, err)
		}
		assert.Equal(t, 1, identityCount(t, userID))
	})

	t.Run("concurrent tenant provisioning creates one identity without grants", func(t *testing.T) {
		tenantID := uuid.New()
		_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, "provisioning")
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := pgPool.Exec(context.Background(), "DELETE FROM users WHERE tenant_id = $1", tenantID)
			require.NoError(t, err)
			_, err = pgPool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)
			require.NoError(t, err)
		})
		email := identityTestEmail(uuid.NewString())
		subject := uuid.NewString()
		const attempts = 8
		users := make(chan models.User, attempts)
		errs := make(chan error, attempts)
		var wg sync.WaitGroup
		for range attempts {
			wg.Go(func() {
				user, err := database.ResolveUserIdentityForTenant(ctx, "tenant-provisioning", subject, email, tenantID)
				users <- user
				errs <- err
			})
		}
		wg.Wait()
		close(users)
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		var userID models.UserId
		for user := range users {
			if userID == "" {
				userID = user.UserId
			}
			require.Equal(t, userID, user.UserId)
			require.Equal(t, tenantID, user.TenantId)
			require.Equal(t, models.NO_ROLE, user.Role)
		}
		require.Equal(t, 1, identityCount(t, string(userID)))
		grants, err := database.ActiveGrantsForPrincipal(ctx, "user", string(userID))
		require.NoError(t, err)
		require.Empty(t, grants)
		t.Run("same subject with concurrent different emails leaves no orphan users", func(t *testing.T) {
			otherSubject := uuid.NewString()
			resolved := make(chan models.User, attempts)
			raceErrors := make(chan error, attempts)
			for range attempts {
				wg.Go(func() {
					u, err := database.ResolveUserIdentityForTenant(ctx, "tenant-provisioning", otherSubject, identityTestEmail(uuid.NewString()), tenantID)
					resolved <- u
					raceErrors <- err
				})
			}
			wg.Wait()
			close(resolved)
			close(raceErrors)
			for err := range raceErrors {
				require.NoError(t, err)
			}
			var first models.UserId
			for u := range resolved {
				if first == "" {
					first = u.UserId
				}
				require.Equal(t, first, u.UserId)
			}
			require.Equal(t, 1, identityCount(t, string(first)))
			var count int
			require.NoError(t, pgPool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE tenant_id = $1", tenantID).Scan(&count))
			require.Equal(t, 2, count)
		})
		_, err = database.ResolveUserIdentityForTenant(ctx, "tenant-provisioning", subject, email, uuid.New())
		require.Error(t, err)
		_, err = pgPool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", userID)
		require.NoError(t, err)
		_, err = database.ResolveUserIdentityForTenant(ctx, "tenant-provisioning", subject, email, tenantID)
		require.ErrorIs(t, err, models.NotFoundError)
	})
}

func createIdentityTestUser(t *testing.T) string {
	t.Helper()

	userID := uuid.NewString()
	query, args, err := postgres.NewQueryBuilder().Insert("users").
		Columns("id", "email", "role").
		Values(userID, identityTestEmail(userID), 0).
		ToSql()
	require.NoError(t, err)
	_, err = pgPool.Exec(t.Context(), query, args...)
	require.NoError(t, err)
	t.Cleanup(func() {
		query, args, err := postgres.NewQueryBuilder().Delete("users").Where(squirrel.Eq{"id": userID}).ToSql()
		require.NoError(t, err)
		_, err = pgPool.Exec(context.Background(), query, args...)
		require.NoError(t, err)
	})
	return userID
}

func identityTestEmail(userID string) string {
	return "identity-" + userID + "@example.test"
}

func identityCount(t *testing.T, userID string) int {
	t.Helper()

	var count int
	query, args, err := postgres.NewQueryBuilder().Select("COUNT(*)").
		From("user_identities").
		Where(squirrel.Eq{"user_id": userID}).
		ToSql()
	require.NoError(t, err)
	err = pgPool.QueryRow(t.Context(), query, args...).Scan(&count)
	require.NoError(t, err)
	return count
}

func grantIdentityTestTenantAccess(t *testing.T, tenantID uuid.UUID, userID string) {
	t.Helper()
	_, err := pgPool.Exec(t.Context(), "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, "identity-"+tenantID.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(t.Context(), `INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
		VALUES ($1, 'user', $2, 'marble', $3, 'TENANT_ADMIN')`, uuid.New(), userID, tenantID)
	require.NoError(t, err)
	_, err = pgPool.Exec(t.Context(), "UPDATE users SET tenant_id = $1 WHERE id = $2", tenantID, userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := pgPool.Exec(context.Background(), "DELETE FROM grants WHERE tenant_id = $1", tenantID)
		require.NoError(t, err)
		_, err = pgPool.Exec(context.Background(), "UPDATE users SET tenant_id = NULL WHERE tenant_id = $1", tenantID)
		require.NoError(t, err)
		_, err = pgPool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)
		require.NoError(t, err)
	})
}
