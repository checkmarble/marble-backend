package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTenantOIDCConfigurationWithAPIKey(t *testing.T) {
	ctx := t.Context()
	tenantID, orgID := uuid.New(), uuid.New()
	userID := uuid.NewString()
	email := userID + "@example.test"

	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().Insert("tenants").
		Columns("id", "name").Values(tenantID, tenantID.String()))
	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().Insert("organizations").
		Columns("id", "name", "tenant_id").Values(orgID, orgID.String(), tenantID))
	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().Insert("users").
		Columns("id", "email", "role", "organization_id").Values(userID, email, int(models.ADMIN), orgID))
	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().Insert("grants").
		Columns("id", "principal_type", "principal_id", "principal_authority", "organization_id", "tenant_id", "role").
		Values(uuid.New(), "user", userID, "marble", orgID, nil, "ADMIN").           // Required for API key creation within an organization
		Values(uuid.New(), "user", userID, "marble", nil, tenantID, "TENANT_ADMIN")) // Required for creating API keys with OIDC_CONFIG_EDITOR role
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, query := range []squirrel.DeleteBuilder{
			repositories.NewQueryBuilder().Delete("audit.audit_events").Where(squirrel.Eq{"tenant_id": tenantID}),
			repositories.NewQueryBuilder().Delete("tenant_oidc_configurations").Where(squirrel.Eq{"tenant_id": tenantID}),
			repositories.NewQueryBuilder().Delete("grants").Where(squirrel.Or{squirrel.Eq{"tenant_id": tenantID}, squirrel.Eq{"organization_id": orgID}}),
			repositories.NewQueryBuilder().Delete("api_keys").Where(squirrel.Eq{"org_id": orgID}),
			repositories.NewQueryBuilder().Delete("users").Where(squirrel.Eq{"id": userID}),
			repositories.NewQueryBuilder().Delete("organizations").Where(squirrel.Eq{"id": orgID}),
			repositories.NewQueryBuilder().Delete("tenants").Where(squirrel.Eq{"id": tenantID}),
		} {
			sql, args, err := query.ToSql()
			require.NoError(t, err)
			_, err = pgPool.Exec(cleanupCtx, sql, args...)
			require.NoError(t, err)
		}
	})

	failedKeyID := uuid.NewString()
	err := testUsecases.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		repo := testUsecases.Repositories.MarbleDbRepository
		if err := repo.CreateApiKey(ctx, tx, models.ApiKey{Id: failedKeyID, OrganizationId: orgID, Role: models.OIDC_CONFIG_EDITOR, Hash: []byte(failedKeyID), Prefix: "err"}); err != nil {
			return err
		}
		return repo.CreateApiKeyGrant(ctx, tx, failedKeyID, models.Grant{Role: models.OIDC_CONFIG_EDITOR, OrganizationId: orgID})
	})
	require.Error(t, err)
	var keyExists bool
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM api_keys WHERE id = $1)", failedKeyID).Scan(&keyExists))
	require.False(t, keyExists)

	e := httpexpect.Default(t, testServer.URL)
	token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(email)).
		Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
	admin := e.Builder(func(req *httpexpect.Request) { req.WithHeader("Authorization", "Bearer "+token) })
	createdKey := admin.POST("/apikeys").WithJSON(map[string]any{"role": "OIDC_CONFIG_EDITOR", "description": "OIDC recovery"}).
		Expect().Status(http.StatusCreated).JSON().Object().Value("api_key").Object()
	keyID := createdKey.Value("id").String().NotEmpty().Raw()
	key := createdKey.Value("key").String().NotEmpty().Raw()
	keyClient := e.Builder(func(req *httpexpect.Request) { req.WithHeader("X-API-Key", key) })

	path := fmt.Sprintf("/tenants/%s/oidc-configuration", tenantID)
	config := map[string]any{"issuer": "https://id.example.test", "client_id": "client", "client_secret": "secret", "sso_enabled": true}
	admin.PUT(path).WithJSON(config).Expect().Status(http.StatusNoContent)
	admin.GET(path).Expect().Status(http.StatusOK).JSON().Object().Value("sso_enabled").Boolean().IsTrue()
	delete(config, "client_secret")
	config["sso_enabled"] = false
	keyClient.PUT(path).WithJSON(config).Expect().Status(http.StatusNoContent)
	admin.GET(path).Expect().Status(http.StatusOK).JSON().Object().Value("sso_enabled").Boolean().IsFalse()

	var configID uuid.UUID
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT id FROM tenant_oidc_configurations WHERE tenant_id = $1", tenantID).Scan(&configID))
	var auditUser, auditKey string
	var insertSecret, updateSecret bool
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT user_id, data ? 'client_secret' FROM audit.audit_events
		WHERE entity_id = $1 AND "table" = 'tenant_oidc_configurations' AND operation = 'INSERT'`, configID).Scan(&auditUser, &insertSecret))
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT api_key_id::text, (data ? 'client_secret' OR previous_data ? 'client_secret') FROM audit.audit_events
		WHERE entity_id = $1 AND "table" = 'tenant_oidc_configurations' AND operation = 'UPDATE'`, configID).Scan(&auditKey, &updateSecret))
	require.Equal(t, userID, auditUser)
	require.Equal(t, keyID, auditKey)
	require.False(t, insertSecret)
	require.False(t, updateSecret)
	admin.DELETE("/apikeys/{api_key_id}", keyID).Expect().Status(http.StatusNoContent)
	keyClient.GET(path).Expect().Status(http.StatusUnauthorized)
	var revoked bool
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM grants WHERE principal_type = 'api_key' AND principal_id = $1 AND tenant_id = $2`, keyID, tenantID).Scan(&revoked))
	require.True(t, revoked)
}
