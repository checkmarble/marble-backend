package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/postgres"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTenantAccessLifecycle(t *testing.T) {
	ctx := utils.StoreLoggerInContext(t.Context(), utils.NewLogger("text"))
	source, target, foreign := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{source, target, foreign} {
		_, err := pgPool.Exec(ctx, "INSERT INTO tenants (id, name) VALUES ($1, $2)", id, id.String())
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for _, id := range []uuid.UUID{source, target, foreign} {
			_, err := pgPool.Exec(context.Background(), "DELETE FROM grants WHERE tenant_id = $1 OR organization_id IN (SELECT id FROM organizations WHERE tenant_id = $1)", id)
			require.NoError(t, err)
			_, err = pgPool.Exec(context.Background(), "DELETE FROM users WHERE tenant_id = $1", id)
			require.NoError(t, err)
			_, err = pgPool.Exec(context.Background(), "DELETE FROM organizations WHERE tenant_id = $1", id)
			require.NoError(t, err)
			_, err = pgPool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", id)
			require.NoError(t, err)
		}
	})
	db := postgres.New(pgPool)
	create := func(tenantID uuid.UUID) models.User {
		id := uuid.NewString()
		u, err := db.ResolveUserIdentityForTenant(ctx, "tenant-access-test", id, identityTestEmail(id), tenantID)
		require.NoError(t, err)
		return u
	}
	actor, pending, zeroGrant, outsider := create(source), create(source), create(source), create(foreign)
	var auditActor string
	var auditTenant uuid.UUID
	var auditOrg *uuid.UUID
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT user_id, tenant_id, org_id FROM audit.audit_events WHERE entity_id = $1 AND "table" = 'users' AND operation = 'INSERT' ORDER BY id DESC LIMIT 1`, pending.UserId).Scan(&auditActor, &auditTenant, &auditOrg))
	require.Equal(t, string(pending.UserId), auditActor)
	require.Equal(t, source, auditTenant)
	require.Nil(t, auditOrg)
	pending, err := db.UpdateUserProfileFromClaims(ctx, pending, models.IdentityUpdatableClaims{Firstname: "Pending", Lastname: "Identity"})
	require.NoError(t, err)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT user_id, tenant_id, org_id FROM audit.audit_events WHERE entity_id = $1 AND "table" = 'users' AND operation = 'UPDATE' ORDER BY id DESC LIMIT 1`, pending.UserId).Scan(&auditActor, &auditTenant, &auditOrg))
	require.Equal(t, string(pending.UserId), auditActor)
	require.Equal(t, source, auditTenant)
	require.Nil(t, auditOrg)
	_, err = pgPool.Exec(ctx, "INSERT INTO grants (id, principal_type, principal_authority, principal_id, tenant_id, role) VALUES ($1, 'user', 'marble', $2, $3, 'TENANT_ADMIN')", uuid.New(), actor.UserId, source)
	require.NoError(t, err)
	credentials := models.Credentials{ActorIdentity: models.Identity{UserId: actor.UserId}, Roles: []models.Role{models.TENANT_ADMIN}, TenantId: source, AuthTenantId: source}
	ctx = context.WithValue(ctx, utils.ContextKeyCredentials, credentials)
	withCreds := generateUsecaseWithCreds(testUsecases, credentials)
	uc := withCreds.NewUserUseCase()
	users, err := uc.ListUsersByTenant(ctx, source)
	require.NoError(t, err)
	require.Len(t, users, 3)
	_, err = uc.ListUsersByTenant(ctx, foreign)
	require.ErrorIs(t, err, models.ForbiddenError)
	var linkedSubject string
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT subject FROM user_identities WHERE user_id = $1", pending.UserId).Scan(&linkedSubject))
	_, err = db.ResolveUserIdentityForTenant(ctx, "tenant-access-test", linkedSubject, pending.Email, foreign)
	require.ErrorIs(t, err, models.NotFoundError)
	_, err = pgPool.Exec(ctx, "UPDATE users SET role = $1 WHERE id = $2", int(models.MARBLE_ADMIN), pending.UserId)
	require.NoError(t, err)
	require.ErrorIs(t, uc.SetTenantAdministrator(ctx, source, string(pending.UserId), true), models.NotFoundError)
	_, err = pgPool.Exec(ctx, "UPDATE users SET role = $1 WHERE id = $2", int(models.NO_ROLE), pending.UserId)
	require.NoError(t, err)
	require.ErrorIs(t, uc.SetTenantAdministrator(ctx, source, string(outsider.UserId), true), models.NotFoundError)
	require.ErrorIs(t, uc.SetTenantAdministrator(ctx, source, string(actor.UserId), false), models.ForbiddenError)
	const attempts = 8
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Go(func() { errs <- uc.SetTenantAdministrator(ctx, source, string(pending.UserId), true) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	grants, err := db.ActiveGrantsForPrincipal(ctx, "user", string(pending.UserId))
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, models.TENANT_ADMIN, grants[0].Role)
	require.Equal(t, uuid.Nil, grants[0].OrganizationId)
	require.NoError(t, uc.SetTenantAdministrator(ctx, source, string(pending.UserId), false))
	grants, err = db.ActiveGrantsForPrincipal(ctx, "user", string(pending.UserId))
	require.NoError(t, err)
	require.Empty(t, grants)
	orgID := uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, 'Tenant access organization', $2)", orgID, source)
	require.NoError(t, err)
	caseID := uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO cases (id, org_id, name, assigned_to) VALUES ($1, $2, 'Identity lookup', $3)", caseID, orgID, zeroGrant.UserId)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO case_events (case_id, org_id, user_id, event_type) VALUES ($1, $2, $3, 'comment')", caseID, orgID, pending.UserId)
	require.NoError(t, err)
	caseUsers, err := testUsecases.Repositories.MarbleDbRepository.ListCaseEventUsers(ctx, testUsecases.NewExecutorFactory().NewExecutor(), caseID.String())
	require.NoError(t, err)
	require.Len(t, caseUsers, 2)
	_, err = pgPool.Exec(ctx, "DELETE FROM cases WHERE id = $1", caseID)
	require.NoError(t, err)
	require.NoError(t, uc.ReplaceOrganizationGrant(ctx, string(pending.UserId), source, orgID, models.VIEWER))
	listed, err := testUsecases.Repositories.MarbleDbRepository.ListUsers(ctx, testUsecases.NewExecutorFactory().NewExecutor(), &orgID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, models.VIEWER, listed[0].Role)
	require.NoError(t, uc.ReplaceOrganizationGrant(ctx, string(pending.UserId), source, orgID, models.ADMIN))
	localCreds := models.Credentials{ActorIdentity: models.Identity{UserId: pending.UserId}, OrganizationId: orgID, TenantId: source, Roles: []models.Role{models.ADMIN}}
	localWithCreds := generateUsecaseWithCreds(testUsecases, localCreds)
	local := localWithCreds.NewUserUseCase()
	require.ErrorIs(t, local.SetTenantAdministrator(ctx, source, string(zeroGrant.UserId), true), models.ForbiddenError)
	require.ErrorIs(t, local.RevokeOrganizationGrant(ctx, string(pending.UserId), source, orgID), models.ForbiddenError)
	require.ErrorIs(t, local.ReplaceOrganizationGrant(ctx, string(pending.UserId), source, orgID, models.VIEWER), models.ForbiddenError)
	require.ErrorIs(t, local.DeleteUser(ctx, string(zeroGrant.UserId), string(pending.UserId)), models.ForbiddenError)
	changedEmail := "changed@example.test"
	_, err = local.UpdateUser(ctx, models.UpdateUser{UserId: string(actor.UserId), Email: &changedEmail})
	require.ErrorIs(t, err, models.ForbiddenError)
	require.ErrorIs(t, uc.ReplaceOrganizationGrant(ctx, string(pending.UserId), source, orgID, models.MARBLE_ADMIN), models.BadParameterError)
	repo := testUsecases.Repositories.MarbleDbRepository
	rollback := errors.New("rollback deletion")
	err = testUsecases.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		if err := repo.DeleteUser(ctx, tx, pending.UserId); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	grants, err = db.ActiveGrantsForPrincipal(ctx, "user", string(pending.UserId))
	require.NoError(t, err)
	require.Len(t, grants, 1)
	errs = make(chan error, attempts+1)
	for range attempts {
		wg.Go(func() { errs <- uc.SetTenantAdministrator(ctx, source, string(pending.UserId), true) })
	}
	wg.Go(func() { errs <- uc.DeleteUser(ctx, string(pending.UserId), string(actor.UserId)) })
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			require.ErrorIs(t, err, models.NotFoundError)
		}
	}
	grants, err = db.ActiveGrantsForPrincipal(ctx, "user", string(pending.UserId))
	require.NoError(t, err)
	require.Empty(t, grants)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT user_id, tenant_id FROM audit.audit_events WHERE entity_id = $1 AND "table" = 'users' AND operation = 'UPDATE' AND data->>'deleted_at' IS NOT NULL ORDER BY id DESC LIMIT 1`, pending.UserId).Scan(&auditActor, &auditTenant))
	require.Equal(t, string(actor.UserId), auditActor)
	require.Equal(t, source, auditTenant)
	require.ErrorIs(t, uc.SetTenantAdministrator(ctx, source, string(pending.UserId), true), models.NotFoundError)
	_, err = db.ResolveUserIdentityForTenant(ctx, "tenant-access-test", string(pending.UserId), pending.Email, source)
	require.ErrorIs(t, err, models.NotFoundError)
	// Prime the identity lookup before merge; a zero-grant identity must move too.
	_, err = repo.UserById(ctx, testUsecases.NewExecutorFactory().NewExecutor(), string(zeroGrant.UserId))
	require.NoError(t, err)
	platform := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	platform.Credentials.ActorIdentity.UserId = actor.UserId
	merge := platform.NewTenantUsecase()
	platformCtx := context.WithValue(ctx, utils.ContextKeyCredentials, platform.Credentials)
	require.NoError(t, merge.Merge(platformCtx, usecases.TenantMergeInput{TargetTenantId: target, SourceTenantIds: []uuid.UUID{source}}))
	loaded, err := repo.UserById(ctx, testUsecases.NewExecutorFactory().NewExecutor(), string(zeroGrant.UserId))
	require.NoError(t, err)
	require.Equal(t, target, loaded.TenantId)
	grants, err = db.ActiveGrantsForPrincipal(ctx, "user", string(zeroGrant.UserId))
	require.NoError(t, err)
	require.Empty(t, grants)
	platformUsers := platform.NewUserUseCase()
	users, err = platformUsers.ListUsersByTenant(ctx, target)
	require.NoError(t, err)
	require.Len(t, users, 2)
	users, err = platformUsers.ListUsersByTenant(ctx, source)
	require.NoError(t, err)
	require.Empty(t, users)
}
