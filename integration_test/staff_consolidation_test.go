package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/cmd/staffconsolidation"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStaffAccountConsolidation(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	clearMarker := func() {
		_, _ = pgPool.Exec(ctx, `DELETE FROM metadata WHERE org_id IS NULL AND key = 'staff_accounts_consolidated'`)
	}
	clearMarker()
	t.Cleanup(clearMarker)

	admin := generateUsecaseWithCredForMarbleAdmin(testUsecases)
	orgUsecase := admin.NewOrganizationUseCase()
	suffix := uuid.NewString()[:8]
	orgAData, err := orgUsecase.CreateOrganization(ctx, models.CreateOrganizationInput{Name: "staff consolidation org a " + suffix})
	require.NoError(t, err)
	orgBData, err := orgUsecase.CreateOrganization(ctx, models.CreateOrganizationInput{Name: "staff consolidation org b " + suffix})
	require.NoError(t, err)
	orgA := orgAData.Id
	orgB := orgBData.Id
	schemaNames := []string{"org-" + orgAData.Name, "org-" + orgBData.Name}
	for _, organizationName := range []string{orgAData.Name, orgBData.Name} {
		_, err = pgPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier{"org-" + organizationName}.Sanitize()))
		require.NoError(t, err)
	}

	plainID := uuid.New()
	plainAliasID := uuid.New()
	plainAliasBID := uuid.New()
	aliasCanonicalID := uuid.New()
	aliasOtherID := uuid.New()
	if aliasOtherID.String() < aliasCanonicalID.String() {
		aliasCanonicalID, aliasOtherID = aliasOtherID, aliasCanonicalID
	}
	inboxID := uuid.New()
	caseID := uuid.New()
	base := "staffintegration" + uuid.NewString()[:8]
	t.Cleanup(func() {
		for _, schemaName := range schemaNames {
			_, _ = pgPool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pgx.Identifier{schemaName}.Sanitize()))
		}
		_, _ = pgPool.Exec(ctx, `DELETE FROM case_events WHERE case_id = $1`, caseID)
		_, _ = pgPool.Exec(ctx, `DELETE FROM cases WHERE id = $1`, caseID)
		_, _ = pgPool.Exec(ctx, `DELETE FROM inboxes WHERE id = $1`, inboxID)
		_, _ = pgPool.Exec(ctx, `DELETE FROM grants WHERE principal_id = ANY($1::text[])`, []string{plainID.String(), plainAliasID.String(), plainAliasBID.String(), aliasCanonicalID.String(), aliasOtherID.String()})
		_, _ = pgPool.Exec(ctx, `DELETE FROM users WHERE email LIKE $1`, base+"%")
	})
	plainEmail := base + "@checkmarble.com"
	plainAliasEmail := base + "+org-a@checkmarble.com"
	plainAliasBEmail := base + "+org-b@checkmarble.com"
	aliasCanonicalEmail := base + "-fallback+first@checkmarble.com"
	canonicalFallbackEmail := base + "-fallback@checkmarble.com"
	aliasOtherEmail := base + "-fallback+second@checkmarble.com"

	_, err = pgPool.Exec(ctx, `
		INSERT INTO users (id, email, role, organization_id) VALUES
			($1, $2, 6, NULL), ($3, $4, 4, $6), ($5, $7, 4, $8),
			($9, $10, 4, $6), ($11, $12, 4, $8)`,
		plainID, plainEmail, plainAliasID, plainAliasEmail, plainAliasBID, orgA, plainAliasBEmail,
		orgB, aliasCanonicalID, aliasCanonicalEmail, aliasOtherID, aliasOtherEmail)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `
		INSERT INTO grants (id, principal_type, principal_id, principal_authority, organization_id, role) VALUES
			($1, 'user', $2, 'marble', $3, 'VIEWER'),
			($4, 'user', $5, 'marble', $6, 'ADMIN'),
			($7, 'user', $8, 'marble', $9, 'PUBLISHER'),
			($10, 'user', $11, 'marble', $9, 'PUBLISHER'),
			($12, 'user', $11, 'marble', $6, 'ANALYST')`,
		uuid.New(), plainAliasID.String(), orgA,
		uuid.New(), plainAliasBID.String(), orgB,
		uuid.New(), aliasCanonicalID.String(), orgA,
		uuid.New(), aliasOtherID.String(),
		uuid.New())
	require.NoError(t, err)

	_, err = pgPool.Exec(ctx, `INSERT INTO inboxes (id, name, organization_id) VALUES ($1, 'staff consolidation inbox', $2)`, inboxID, orgA)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO cases (id, org_id, name, inbox_id) VALUES ($1, $2, 'staff consolidation case', $3)`, caseID, orgA, inboxID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO inbox_users (inbox_id, user_id) VALUES ($1, $2), ($1, $3)`, inboxID, plainAliasID, plainAliasBID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO case_contributors (case_id, user_id) VALUES ($1, $2), ($1, $3)`, caseID, plainAliasID, plainAliasBID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO case_events (id, case_id, user_id, event_type, new_value) VALUES ($1, $2, $3, 'case_assigned', $4)`, uuid.New(), caseID, plainAliasID, plainAliasID.String())
	require.NoError(t, err)

	clientExecutor := func(ctx context.Context, organizationID uuid.UUID) (repositories.Executor, error) {
		return admin.NewExecutorFactory().NewClientDbExecutor(ctx, organizationID)
	}
	initialReport, err := staffconsolidation.Discover(ctx, pgPool)
	require.NoError(t, err)
	require.Len(t, initialReport.Groups, 2)
	plainAuditIDs := make(map[uuid.UUID]uuid.UUID, 2)
	fallbackAuditIDs := make(map[uuid.UUID]uuid.UUID, 2)
	for _, organizationID := range []uuid.UUID{orgA, orgB} {
		clientExec, err := clientExecutor(ctx, organizationID)
		require.NoError(t, err)
		table := pgx.Identifier{clientExec.DatabaseSchema().Schema, "_monitored_objects_audit"}.Sanitize()
		_, err = clientExec.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (id uuid primary key, user_id uuid, created_at timestamptz not null default now())`, table))
		require.NoError(t, err)
		plainAuditID := uuid.New()
		plainAuditIDs[organizationID] = plainAuditID
		if organizationID == orgB {
			fallbackAuditID := uuid.New()
			fallbackAuditIDs[organizationID] = fallbackAuditID
			_, err = clientExec.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (id, user_id) VALUES ($1, $2), ($3, $4)`, table), plainAuditID, plainAliasID, fallbackAuditID, aliasOtherID)
		} else {
			_, err = clientExec.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (id, user_id) VALUES ($1, $2)`, table), plainAuditID, plainAliasID)
		}
		require.NoError(t, err)
	}

	auditCalls := 0
	failingClientExecutor := func(ctx context.Context, organizationID uuid.UUID) (repositories.Executor, error) {
		auditCalls++
		if auditCalls == 2 {
			return nil, errors.New("simulated client audit failure")
		}
		return clientExecutor(ctx, organizationID)
	}
	require.Error(t, staffconsolidation.RunOnce(ctx, staffconsolidation.Config{
		MarblePool:     pgPool,
		ClientExecutor: failingClientExecutor,
	}))
	var deleted bool
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id = $1`, plainAliasID).Scan(&deleted))
	require.False(t, deleted, "client audit failure must happen before Marble changes")

	_, err = staffconsolidation.Apply(ctx, staffconsolidation.Config{MarblePool: pgPool}, staffconsolidation.Report{
		Groups: []staffconsolidation.GroupReport{
			{
				CanonicalEmail: plainEmail,
				CanonicalID:    plainID.String(),
				AliasIDs:       []string{plainAliasID.String(), plainAliasBID.String()},
				AliasUUIDs:     []uuid.UUID{plainAliasID, plainAliasBID},
				HasActiveAlias: true,
			},
			{
				CanonicalEmail: "invalid@checkmarble.com",
				CanonicalID:    "not-a-uuid",
				AliasIDs:       []string{aliasCanonicalID.String()},
				AliasUUIDs:     []uuid.UUID{aliasCanonicalID},
				HasActiveAlias: true,
			},
		},
	})
	require.Error(t, err)
	for _, userID := range []uuid.UUID{plainAliasID, plainAliasBID} {
		var deleted bool
		require.NoError(t, pgPool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id = $1`, userID).Scan(&deleted))
		require.False(t, deleted, "the failed transaction must roll back all user changes")
	}

	firebase := &mocks.FirebaseAdminClient{}
	firebaseEmails := make([]string, 0, 2)
	firebase.On("EnsureUser", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).
		Times(2).
		Run(func(args mock.Arguments) { firebaseEmails = append(firebaseEmails, args.String(1)) })

	require.NoError(t, staffconsolidation.RunOnce(ctx, staffconsolidation.Config{
		MarblePool:     pgPool,
		ClientExecutor: clientExecutor,
		FirebaseAdmin:  firebase,
	}))
	require.NoError(t, staffconsolidation.RunOnce(ctx, staffconsolidation.Config{
		MarblePool:     pgPool,
		ClientExecutor: clientExecutor,
		FirebaseAdmin:  firebase,
	}))
	firebase.AssertExpectations(t)
	require.ElementsMatch(t, []string{plainEmail, canonicalFallbackEmail}, firebaseEmails)

	require.NoError(t, pgPool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id = $1`, plainAliasID).Scan(&deleted))
	require.True(t, deleted)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id = $1`, plainAliasBID).Scan(&deleted))
	require.True(t, deleted)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM users WHERE id = $1`, aliasOtherID).Scan(&deleted))
	require.True(t, deleted)

	var activeGrants int
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT count(*) FROM grants WHERE principal_id = $1 AND revoked_at IS NULL`, plainID.String()).Scan(&activeGrants))
	require.Equal(t, 2, activeGrants)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT count(*) FROM grants WHERE principal_id = $1 AND revoked_at IS NULL`, aliasCanonicalID.String()).Scan(&activeGrants))
	require.Equal(t, 2, activeGrants)

	var memberships int
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT count(*) FROM inbox_users WHERE inbox_id = $1 AND user_id = $2`, inboxID, plainID).Scan(&memberships))
	require.Equal(t, 1, memberships)
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT count(*) FROM case_contributors WHERE case_id = $1 AND user_id = $2`, caseID, plainID).Scan(&memberships))
	require.Equal(t, 1, memberships)

	var eventUser, eventValue string
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT user_id::text, new_value FROM case_events WHERE case_id = $1`, caseID).Scan(&eventUser, &eventValue))
	require.Equal(t, plainID.String(), eventUser)
	require.Equal(t, plainID.String(), eventValue)
	for organizationID, auditID := range plainAuditIDs {
		clientExec, err := clientExecutor(ctx, organizationID)
		require.NoError(t, err)
		table := pgx.Identifier{clientExec.DatabaseSchema().Schema, "_monitored_objects_audit"}.Sanitize()
		var auditUserID uuid.UUID
		require.NoError(t, clientExec.QueryRow(ctx, fmt.Sprintf(`SELECT user_id FROM %s WHERE id = $1`, table), auditID).Scan(&auditUserID))
		require.Equal(t, plainID, auditUserID)
	}
	clientExec, err := clientExecutor(ctx, orgB)
	require.NoError(t, err)
	table := pgx.Identifier{clientExec.DatabaseSchema().Schema, "_monitored_objects_audit"}.Sanitize()
	var fallbackAuditUserID uuid.UUID
	require.NoError(t, clientExec.QueryRow(ctx, fmt.Sprintf(`SELECT user_id FROM %s WHERE id = $1`, table), fallbackAuditIDs[orgB]).Scan(&fallbackAuditUserID))
	require.Equal(t, aliasCanonicalID, fallbackAuditUserID)

	var marker string
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT value FROM metadata WHERE key = 'staff_accounts_consolidated'`).Scan(&marker))
	require.Equal(t, "true", marker)
}
