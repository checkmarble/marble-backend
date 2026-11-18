package integration

import (
	"context"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTenantOIDCDiscovery(t *testing.T) {
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	createTenantOIDCDiscoveryFixture(t, tenantID, "example.test", true)
	createTenantOIDCDiscoveryFixture(t, otherTenantID, "disabled.example.test", false)

	usecase := testUsecases.NewTenantOIDCDiscoveryUsecase()

	t.Run("discovers an enabled tenant by its exact domain", func(t *testing.T) {
		configuration, err := usecase.ByDomain(t.Context(), " Example.TEST. ")

		require.NoError(t, err)
		require.Equal(t, tenantID, configuration.TenantId)
		require.Equal(t, "https://id.example.test", configuration.Issuer)
		require.Equal(t, "tenant-client", configuration.ClientId)
		require.Empty(t, configuration.ClientSecret)
	})

	t.Run("does not discover a subdomain", func(t *testing.T) {
		_, err := usecase.ByDomain(t.Context(), "login.example.test")

		require.ErrorIs(t, err, models.NotFoundError)
	})

	t.Run("does not discover a disabled tenant configuration", func(t *testing.T) {
		_, err := usecase.ByDomain(t.Context(), "disabled.example.test")

		require.ErrorIs(t, err, models.NotFoundError)
	})
}

func TestTenantDomainVerification(t *testing.T) {
	ctx := t.Context()
	tenantID, otherTenantID := uuid.New(), uuid.New()
	createTenantOIDCDiscoveryFixture(t, tenantID, "verified.example.test", true)
	createTenantOIDCDiscoveryFixture(t, otherTenantID, "other.example.test", true)
	repo := testUsecases.Repositories.MarbleDbRepository
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	pending := models.TenantDomain{Id: uuid.New(), TenantId: tenantID, Domain: "pending.example.test"}
	require.NoError(t, repo.CreateTenantDomain(ctx, exec, pending))
	require.ErrorIs(t, repo.CreateTenantDomain(ctx, exec, models.TenantDomain{
		Id: uuid.New(), TenantId: tenantID, Domain: pending.Domain,
	}), models.ConflictError)

	domains, err := repo.ListTenantDomains(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Len(t, domains, 2)
	require.Nil(t, domains[0].VerifiedAt)
	config, err := repo.GetTenantOIDCConfigurationForAuthentication(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Equal(t, []string{"@verified.example.test"}, config.AllowedDomains)
	_, err = repo.DiscoverTenantOIDCConfigurationByDomain(ctx, exec, pending.Domain)
	require.ErrorIs(t, err, models.NotFoundError)

	for _, domain := range domains {
		if domain.VerifiedAt != nil {
			require.NoError(t, repo.DeleteTenantDomain(ctx, exec, tenantID, domain.Id))
		}
	}
	config, err = repo.GetTenantOIDCConfigurationForAuthentication(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Empty(t, config.AllowedDomains)

	require.ErrorIs(t, repo.SetTenantDomainVerified(ctx, exec, otherTenantID, pending.Id, true), models.NotFoundError)
	require.NoError(t, repo.SetTenantDomainVerified(ctx, exec, tenantID, pending.Id, true))
	domains, err = repo.ListTenantDomains(ctx, exec, tenantID)
	require.NoError(t, err)
	require.NotNil(t, domains[0].VerifiedAt)
	verifiedAt := *domains[0].VerifiedAt
	require.NoError(t, repo.SetTenantDomainVerified(ctx, exec, tenantID, pending.Id, true))
	domains, err = repo.ListTenantDomains(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Equal(t, verifiedAt, *domains[0].VerifiedAt)
	config, err = repo.GetTenantOIDCConfigurationForAuthentication(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Equal(t, []string{"@pending.example.test"}, config.AllowedDomains)
	discovered, err := repo.DiscoverTenantOIDCConfigurationByDomain(ctx, exec, pending.Domain)
	require.NoError(t, err)
	require.Equal(t, tenantID, discovered.TenantId)

	conflict := models.TenantDomain{Id: uuid.New(), TenantId: otherTenantID, Domain: pending.Domain}
	require.NoError(t, repo.CreateTenantDomain(ctx, exec, conflict))
	require.ErrorIs(t, repo.SetTenantDomainVerified(ctx, exec, otherTenantID, conflict.Id, true), models.ConflictError)
	require.NoError(t, repo.SetTenantDomainVerified(ctx, exec, tenantID, pending.Id, false))
	domains, err = repo.ListTenantDomains(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Nil(t, domains[0].VerifiedAt)
	config, err = repo.GetTenantOIDCConfigurationForAuthentication(ctx, exec, tenantID)
	require.NoError(t, err)
	require.Empty(t, config.AllowedDomains)
	_, err = repo.DiscoverTenantOIDCConfigurationByDomain(ctx, exec, pending.Domain)
	require.ErrorIs(t, err, models.NotFoundError)
	require.NoError(t, repo.SetTenantDomainVerified(ctx, exec, tenantID, pending.Id, false))
	require.NoError(t, repo.SetTenantDomainVerified(ctx, exec, tenantID, pending.Id, true))
	discovered, err = repo.DiscoverTenantOIDCConfigurationByDomain(ctx, exec, pending.Domain)
	require.NoError(t, err)
	require.Equal(t, tenantID, discovered.TenantId)
}

func createTenantOIDCDiscoveryFixture(t *testing.T, tenantID uuid.UUID, domain string, ssoEnabled bool) {
	t.Helper()
	ctx := t.Context()

	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().
		Insert("tenants").
		Columns("id", "name").
		Values(tenantID, "oidc-"+tenantID.String()))
	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().
		Insert("tenant_oidc_configurations").
		Columns("id", "tenant_id", "issuer", "client_id", "client_secret", "scopes", "extra_params", "sso_enabled").
		Values(uuid.New(), tenantID, "https://id.example.test", "tenant-client", []byte("encrypted-secret"), []string{"openid", "email"}, []byte(`{}`), ssoEnabled))
	insertTenantOIDCDiscoveryFixture(t, ctx, repositories.NewQueryBuilder().
		Insert("tenant_domains").
		Columns("id", "tenant_id", "domain", "verified_at").
		Values(uuid.New(), tenantID, domain, squirrel.Expr("now()")))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, queryBuilder := range []squirrel.DeleteBuilder{
			repositories.NewQueryBuilder().Delete("tenant_domains").Where(squirrel.Eq{"tenant_id": tenantID}),
			repositories.NewQueryBuilder().Delete("tenant_oidc_configurations").Where(squirrel.Eq{"tenant_id": tenantID}),
			repositories.NewQueryBuilder().Delete("tenants").Where(squirrel.Eq{"id": tenantID}),
		} {
			query, args, err := queryBuilder.ToSql()
			require.NoError(t, err)
			_, err = pgPool.Exec(cleanupCtx, query, args...)
			require.NoError(t, err)
		}
	})
}

func insertTenantOIDCDiscoveryFixture(t *testing.T, ctx context.Context, query squirrel.InsertBuilder) {
	t.Helper()
	sql, args, err := query.ToSql()
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, sql, args...)
	require.NoError(t, err)
}
