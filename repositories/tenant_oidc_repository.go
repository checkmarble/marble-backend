package repositories

import (
	"context"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TenantOIDCRepository interface {
	GetTenantOIDCConfiguration(ctx context.Context, exec Executor, tenantId uuid.UUID) (models.TenantOIDCConfiguration, error)
	GetTenantOIDCConfigurationForAuthentication(ctx context.Context, exec Executor, tenantId uuid.UUID) (models.TenantOIDCConfiguration, error)
	UpsertTenantOIDCConfiguration(ctx context.Context, exec Executor, config models.TenantOIDCConfiguration) error
	DeleteTenantOIDCConfiguration(ctx context.Context, exec Executor, tenantId uuid.UUID) error
	ListTenantDomains(ctx context.Context, exec Executor, tenantId uuid.UUID) ([]models.TenantDomain, error)
	CreateTenantDomain(ctx context.Context, exec Executor, domain models.TenantDomain) error
	DeleteTenantDomain(ctx context.Context, exec Executor, tenantId, domainId uuid.UUID) error
	DiscoverTenantOIDCConfigurationByDomain(ctx context.Context, exec Executor, domain string) (models.TenantOIDCConfiguration, error)
}

func (repo *MarbleDbRepository) GetTenantOIDCConfiguration(ctx context.Context, exec Executor, tenantId uuid.UUID) (models.TenantOIDCConfiguration, error) {
	return repo.getTenantOIDCConfiguration(ctx, exec, tenantId, false)
}

func (repo *MarbleDbRepository) GetTenantOIDCConfigurationForAuthentication(ctx context.Context, exec Executor, tenantId uuid.UUID) (models.TenantOIDCConfiguration, error) {
	return repo.getTenantOIDCConfiguration(ctx, exec, tenantId, true)
}

func (repo *MarbleDbRepository) getTenantOIDCConfiguration(ctx context.Context, exec Executor, tenantId uuid.UUID, withSecret bool) (models.TenantOIDCConfiguration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.TenantOIDCConfiguration{}, err
	}

	var encryptedSecret, extraParams []byte
	config, err := SqlToRow(ctx, exec, NewQueryBuilder().
		Select("tenant_id", "issuer", "client_id", "client_secret", "scopes", "extra_params", "email_claim", "sso_enabled", "created_at", "updated_at").
		From("tenant_oidc_configurations").
		Where(squirrel.Eq{"tenant_id": tenantId}), func(row pgx.CollectableRow) (models.TenantOIDCConfiguration, error) {
		var config models.TenantOIDCConfiguration
		if err := row.Scan(&config.TenantId, &config.Issuer, &config.ClientId, &encryptedSecret, &config.Scopes, &extraParams,
			&config.EmailClaim, &config.SsoEnabled, &config.CreatedAt, &config.UpdatedAt); err != nil {
			return config, err
		}
		if err := json.Unmarshal(extraParams, &config.ExtraParams); err != nil {
			return config, errors.Wrap(err, "decoding tenant OIDC extra parameters")
		}
		return config, nil
	})
	if err != nil {
		return models.TenantOIDCConfiguration{}, err
	}
	if !withSecret {
		return config, nil
	}
	if repo.oidcSecretsKeeper == nil {
		return models.TenantOIDCConfiguration{}, errors.New("OIDC secrets keeper is not configured")
	}
	secret, err := repo.oidcSecretsKeeper.Decrypt(ctx, encryptedSecret)
	if err != nil {
		return models.TenantOIDCConfiguration{}, errors.Wrap(err, "decrypting tenant OIDC client secret")
	}
	config.ClientSecret = string(secret)
	domains, err := repo.ListTenantDomains(ctx, exec, tenantId)
	if err != nil {
		return models.TenantOIDCConfiguration{}, err
	}
	config.AllowedDomains = make([]string, 0, len(domains))
	for _, domain := range domains {
		config.AllowedDomains = append(config.AllowedDomains, "@"+domain.Domain)
	}
	return config, nil
}

func (repo *MarbleDbRepository) UpsertTenantOIDCConfiguration(ctx context.Context, exec Executor, config models.TenantOIDCConfiguration) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if repo.oidcSecretsKeeper == nil {
		return errors.New("OIDC secrets keeper is not configured")
	}
	secret, err := repo.oidcSecretsKeeper.Encrypt(ctx, []byte(config.ClientSecret))
	if err != nil {
		return errors.Wrap(err, "encrypting tenant OIDC client secret")
	}
	extraParams, err := json.Marshal(config.ExtraParams)
	if err != nil {
		return errors.Wrap(err, "encoding tenant OIDC extra parameters")
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().
		Insert("tenant_oidc_configurations").
		Columns("id", "tenant_id", "issuer", "client_id", "client_secret", "scopes", "extra_params", "email_claim", "sso_enabled").
		Values(pure_utils.NewId(), config.TenantId, config.Issuer, config.ClientId, secret, config.Scopes, extraParams, config.EmailClaim, config.SsoEnabled).
		Suffix(`ON CONFLICT (tenant_id) DO UPDATE SET issuer = EXCLUDED.issuer, client_id = EXCLUDED.client_id,
			client_secret = EXCLUDED.client_secret, scopes = EXCLUDED.scopes, extra_params = EXCLUDED.extra_params,
			email_claim = EXCLUDED.email_claim, sso_enabled = EXCLUDED.sso_enabled, updated_at = now()`))
}

func (repo *MarbleDbRepository) DeleteTenantOIDCConfiguration(ctx context.Context, exec Executor, tenantId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().Delete("tenant_oidc_configurations").Where(squirrel.Eq{"tenant_id": tenantId}))
}

func (repo *MarbleDbRepository) ListTenantDomains(ctx context.Context, exec Executor, tenantId uuid.UUID) ([]models.TenantDomain, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	return SqlToListOfRow(ctx, exec, NewQueryBuilder().
		Select("id", "tenant_id", "domain", "verified_at", "created_at").
		From("tenant_domains").
		Where(squirrel.Eq{"tenant_id": tenantId}).
		OrderBy("domain"), func(row pgx.CollectableRow) (models.TenantDomain, error) {
		var domain models.TenantDomain
		return domain, row.Scan(&domain.Id, &domain.TenantId, &domain.Domain, &domain.VerifiedAt, &domain.CreatedAt)
	})
}

func (repo *MarbleDbRepository) CreateTenantDomain(ctx context.Context, exec Executor, domain models.TenantDomain) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	err := ExecBuilder(ctx, exec, NewQueryBuilder().Insert("tenant_domains").
		Columns("id", "tenant_id", "domain", "verified_at").
		Values(domain.Id, domain.TenantId, domain.Domain, domain.VerifiedAt))
	if IsUniqueViolationErrorOf(err, "tenant_domains_verified_domain_unique_idx") {
		return models.ConflictError
	}
	return err
}

func (repo *MarbleDbRepository) DeleteTenantDomain(ctx context.Context, exec Executor, tenantId, domainId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	return ExecBuilder(ctx, exec, NewQueryBuilder().Delete("tenant_domains").
		Where(squirrel.Eq{"id": domainId, "tenant_id": tenantId}))
}

func (repo *MarbleDbRepository) HasOIDCConfigurationOrDomains(ctx context.Context, exec Executor, tenantIds []uuid.UUID) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	var found bool
	err := exec.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM tenant_oidc_configurations WHERE tenant_id = ANY($1)
		UNION ALL SELECT 1 FROM tenant_domains WHERE tenant_id = ANY($1)
	)`, tenantIds).Scan(&found)
	return found, err
}

func (repo *MarbleDbRepository) DiscoverTenantOIDCConfigurationByDomain(ctx context.Context, exec Executor, domain string) (models.TenantOIDCConfiguration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.TenantOIDCConfiguration{}, err
	}
	return SqlToRow(ctx, exec, NewQueryBuilder().
		Select("c.tenant_id", "c.issuer", "c.client_id", "c.scopes", "c.extra_params").
		From("tenant_domains d").
		Join("tenant_oidc_configurations c ON c.tenant_id = d.tenant_id").
		Join("tenants t ON t.id = c.tenant_id").
		Where(squirrel.Eq{"c.sso_enabled": true, "t.deleted_at": nil}).
		Where(squirrel.Eq{"d.domain": domain}).
		Limit(1), func(row pgx.CollectableRow) (models.TenantOIDCConfiguration, error) {
		var config models.TenantOIDCConfiguration
		var extraParams []byte
		if err := row.Scan(&config.TenantId, &config.Issuer, &config.ClientId, &config.Scopes, &extraParams); err != nil {
			return config, err
		}
		if err := json.Unmarshal(extraParams, &config.ExtraParams); err != nil {
			return config, errors.Wrap(err, "decoding tenant OIDC extra parameters")
		}
		return config, nil
	})
}
