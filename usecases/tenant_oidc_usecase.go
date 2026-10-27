package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TenantOIDCConfigurationInput struct {
	Issuer       string
	ClientId     string
	ClientSecret *string
	Scopes       []string
	ExtraParams  map[string]string
	EmailClaim   string
	SsoEnabled   bool
}

var defaultTenantOIDCScopes = []string{"openid", "profile", "email"}

type TenantOIDCUsecase struct {
	enforceSecurity    security.EnforceSecurity
	transactionFactory executor_factory.TransactionFactory
	executorFactory    executor_factory.ExecutorFactory
	repository         repositories.TenantOIDCRepository
	tenantRepository   repositories.TenantRepository
	ssoEnabled         bool
	allowHTTP          bool
}

func NewTenantOIDCUsecase(
	enforceSecurity security.EnforceSecurity,
	transactionFactory executor_factory.TransactionFactory,
	executorFactory executor_factory.ExecutorFactory,
	repository repositories.TenantOIDCRepository,
	tenantRepository repositories.TenantRepository,
	ssoEnabled bool,
	allowHTTP bool,
) TenantOIDCUsecase {
	return TenantOIDCUsecase{
		enforceSecurity:    enforceSecurity,
		transactionFactory: transactionFactory,
		executorFactory:    executorFactory,
		repository:         repository,
		tenantRepository:   tenantRepository,
		ssoEnabled:         ssoEnabled,
		allowHTTP:          allowHTTP,
	}
}

func (usecase TenantOIDCUsecase) GetConfiguration(ctx context.Context, tenantId uuid.UUID) (models.TenantOIDCConfiguration, error) {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return models.TenantOIDCConfiguration{}, err
	}
	return usecase.repository.GetTenantOIDCConfiguration(ctx, usecase.executorFactory.NewExecutor(), tenantId)
}

func (usecase TenantOIDCUsecase) SaveConfiguration(ctx context.Context, tenantId uuid.UUID, input TenantOIDCConfigurationInput) error {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return err
	}
	if !usecase.ssoEnabled {
		return errors.Wrap(models.ForbiddenError, "SSO is not enabled by the deployment license")
	}
	if err := validateTenantOIDCConfiguration(input, usecase.allowHTTP); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateUpdate(ctx, tx, tenantId); err != nil {
			return err
		}
		secret := ""
		if input.ClientSecret != nil {
			secret = *input.ClientSecret
		} else {
			current, err := usecase.repository.GetTenantOIDCConfigurationForAuthentication(ctx, tx, tenantId)
			if err != nil {
				if errors.Is(err, models.NotFoundError) {
					return errors.Wrap(models.BadParameterError, "OIDC client secret is required for a new configuration")
				}
				return errors.Wrap(err, "loading existing tenant OIDC client secret")
			}
			if current.Issuer != strings.TrimSpace(input.Issuer) || current.ClientId != strings.TrimSpace(input.ClientId) {
				return errors.Wrap(models.BadParameterError, "OIDC client secret is required when issuer or client ID changes")
			}
			secret = current.ClientSecret
		}
		return usecase.repository.UpsertTenantOIDCConfiguration(ctx, tx, models.TenantOIDCConfiguration{
			TenantId:     tenantId,
			Issuer:       strings.TrimSpace(input.Issuer),
			ClientId:     strings.TrimSpace(input.ClientId),
			ClientSecret: secret,
			Scopes:       tenantOIDCScopes(input.Scopes),
			ExtraParams:  input.ExtraParams,
			EmailClaim:   strings.TrimSpace(input.EmailClaim),
			SsoEnabled:   input.SsoEnabled,
		})
	})
}

func tenantOIDCScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return append([]string(nil), defaultTenantOIDCScopes...)
	}
	return scopes
}

func (usecase TenantOIDCUsecase) DeleteConfiguration(ctx context.Context, tenantId uuid.UUID) error {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return err
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateUpdate(ctx, tx, tenantId); err != nil {
			return err
		}
		return usecase.repository.DeleteTenantOIDCConfiguration(ctx, tx, tenantId)
	})
}

func (usecase TenantOIDCUsecase) ListDomains(ctx context.Context, tenantId uuid.UUID) ([]models.TenantDomain, error) {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return nil, err
	}
	return usecase.repository.ListTenantDomains(ctx, usecase.executorFactory.NewExecutor(), tenantId)
}

func (usecase TenantOIDCUsecase) AddDomain(ctx context.Context, tenantId uuid.UUID, domain string) (models.TenantDomain, error) {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return models.TenantDomain{}, err
	}
	domain, err := normalizeTenantDomain(domain)
	if err != nil {
		return models.TenantDomain{}, err
	}
	now := time.Now()
	entry := models.TenantDomain{Id: pure_utils.NewId(), TenantId: tenantId, Domain: domain, VerifiedAt: now, CreatedAt: now}
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateUpdate(ctx, tx, tenantId); err != nil {
			return err
		}
		return usecase.repository.CreateTenantDomain(ctx, tx, entry)
	})
	return entry, err
}

func (usecase TenantOIDCUsecase) DeleteDomain(ctx context.Context, tenantId, domainId uuid.UUID) error {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return err
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateUpdate(ctx, tx, tenantId); err != nil {
			return err
		}
		return usecase.repository.DeleteTenantDomain(ctx, tx, tenantId, domainId)
	})
}

func validateTenantOIDCConfiguration(input TenantOIDCConfigurationInput, allowHTTP bool) error {
	if err := infra.ValidateTenantOIDCURL(strings.TrimSpace(input.Issuer), allowHTTP); err != nil {
		return err
	}
	if strings.TrimSpace(input.ClientId) == "" {
		return errors.Wrap(models.BadParameterError, "OIDC client ID is required")
	}
	if input.ClientSecret != nil && strings.TrimSpace(*input.ClientSecret) == "" {
		return errors.Wrap(models.BadParameterError, "OIDC client secret cannot be empty")
	}
	return nil
}

func normalizeTenantDomain(raw string) (string, error) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if !isValidTenantDomain(domain) {
		return "", errors.Wrap(models.BadParameterError, "invalid domain")
	}
	return domain, nil
}

func isValidTenantDomain(domain string) bool {
	if domain == "" || len(domain) > 253 || !strings.Contains(domain, ".") {
		return false
	}
	for label := range strings.SplitSeq(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' {
				return false
			}
		}
	}
	return true
}
