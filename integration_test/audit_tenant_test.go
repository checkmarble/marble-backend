package integration

import (
	"context"
	"encoding/json"
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

	_, err = testUsecases.NewExecutorFactory().NewExecutor().Exec(ctx, `
		INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, role)
		VALUES ($1, 'user', 'audit-test-principal', 'marble', $2, 'TENANT_ADMIN')
	`, grantID, tenantID)
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

func TestTenantOIDCAuditRedactsSecret(t *testing.T) {
	tenantID, configID := uuid.New(), uuid.New()
	ctx := context.WithValue(t.Context(), utils.ContextKeyCredentials, models.Credentials{
		ActorIdentity:  models.Identity{UserId: "audit-test-user"},
		OrganizationId: uuid.New(), TenantId: tenantID,
	})
	_, err := pgPool.Exec(t.Context(), "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pgPool.Exec(context.Background(), "DELETE FROM tenant_oidc_configurations WHERE tenant_id = $1", tenantID)
		_, _ = pgPool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)
		_, _ = pgPool.Exec(context.Background(), `DELETE FROM audit.audit_events WHERE entity_id = $1`, configID)
	})
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	_, err = exec.Exec(ctx, `INSERT INTO tenant_oidc_configurations (id, tenant_id, issuer, client_id, client_secret) VALUES ($1, $2, $3, $4, $5)`, configID, tenantID, "https://id.example.test", "client", []byte("encrypted"))
	require.NoError(t, err)
	_, err = exec.Exec(ctx, `UPDATE tenant_oidc_configurations SET client_secret = $1 WHERE id = $2`, []byte("rotated"), configID)
	require.NoError(t, err)
	_, err = exec.Exec(ctx, `DELETE FROM tenant_oidc_configurations WHERE id = $1`, configID)
	require.NoError(t, err)
	rows, err := pgPool.Query(t.Context(), `SELECT operation, data, previous_data FROM audit.audit_events WHERE entity_id = $1 ORDER BY created_at, id`, configID)
	require.NoError(t, err)
	defer rows.Close()
	operations := map[string]bool{}
	for rows.Next() {
		var operation string
		var data, previous []byte
		require.NoError(t, rows.Scan(&operation, &data, &previous))
		operations[operation] = true
		for _, raw := range [][]byte{data, previous} {
			if len(raw) == 0 {
				continue
			}
			var value map[string]any
			require.NoError(t, json.Unmarshal(raw, &value))
			require.NotContains(t, value, "client_secret")
			require.Equal(t, "client", value["client_id"])
		}
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true}, operations)
}

type auditTestKeeper struct{}

func (auditTestKeeper) Encrypt(_ context.Context, data []byte) ([]byte, error) {
	return append([]byte("encrypted:"), data...), nil
}
func (auditTestKeeper) Decrypt(_ context.Context, data []byte) ([]byte, error) {
	return data[len("encrypted:"):], nil
}

func TestTenantOIDCRepositoryGeneratesStableV7ID(t *testing.T) {
	tenantID := uuid.New()
	_, err := pgPool.Exec(t.Context(), "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantID, tenantID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pgPool.Exec(context.Background(), "DELETE FROM tenant_oidc_configurations WHERE tenant_id = $1", tenantID)
		_, _ = pgPool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", tenantID)
	})
	repo := repositories.NewMarbleDbRepository(false, 0, auditTestKeeper{})
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	config := models.TenantOIDCConfiguration{TenantId: tenantID, Issuer: "https://id.example.test", ClientId: "client", ClientSecret: "secret", Scopes: []string{"openid"}}
	require.NoError(t, repo.UpsertTenantOIDCConfiguration(t.Context(), exec, config))
	var first uuid.UUID
	var encrypted []byte
	require.NoError(t, pgPool.QueryRow(t.Context(), "SELECT id, client_secret FROM tenant_oidc_configurations WHERE tenant_id = $1", tenantID).Scan(&first, &encrypted))
	require.Equal(t, uuid.Version(7), first.Version())
	require.NotEqual(t, []byte("secret"), encrypted)
	config.ClientId = "updated"
	require.NoError(t, repo.UpsertTenantOIDCConfiguration(t.Context(), exec, config))
	var second uuid.UUID
	require.NoError(t, pgPool.QueryRow(t.Context(), "SELECT id FROM tenant_oidc_configurations WHERE tenant_id = $1", tenantID).Scan(&second))
	require.Equal(t, first, second)
	loaded, err := repo.GetTenantOIDCConfigurationForAuthentication(t.Context(), exec, tenantID)
	require.NoError(t, err)
	require.Equal(t, "secret", loaded.ClientSecret)
	require.Equal(t, "updated", loaded.ClientId)
	var hasDefault bool
	require.NoError(t, pgPool.QueryRow(t.Context(), `SELECT column_default IS NOT NULL FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'tenant_oidc_configurations' AND column_name = 'id'`).Scan(&hasDefault))
	require.False(t, hasDefault)
}
