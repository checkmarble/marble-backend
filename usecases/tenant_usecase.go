package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TenantMergeInput struct {
	TargetTenantId  uuid.UUID
	SourceTenantIds []uuid.UUID
	NewName         *string
}

type TenantUsecase struct {
	enforceSecurity        security.EnforceSecurity
	transactionFactory     executor_factory.TransactionFactory
	executorFactory        executor_factory.ExecutorFactory
	tenantRepository       repositories.TenantRepository
	grantRepository        repositories.GrantRepository
	organizationRepository repositories.OrganizationRepository
}

func NewTenantUsecase(
	enforceSecurity security.EnforceSecurity,
	transactionFactory executor_factory.TransactionFactory,
	executorFactory executor_factory.ExecutorFactory,
	tenantRepository repositories.TenantRepository,
	grantRepository repositories.GrantRepository,
	organizationRepository repositories.OrganizationRepository,
) TenantUsecase {
	return TenantUsecase{
		enforceSecurity:        enforceSecurity,
		transactionFactory:     transactionFactory,
		executorFactory:        executorFactory,
		tenantRepository:       tenantRepository,
		grantRepository:        grantRepository,
		organizationRepository: organizationRepository,
	}
}

func (usecase TenantUsecase) List(ctx context.Context) ([]models.Tenant, error) {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_LIST); err != nil {
		return nil, err
	}
	return usecase.tenantRepository.ListActive(ctx, usecase.executorFactory.NewExecutor())
}

func (usecase TenantUsecase) UpdateName(ctx context.Context, tenantId uuid.UUID, name string) error {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_UPDATE); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return errors.Wrap(models.BadParameterError, "tenant name cannot be empty")
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateUpdate(ctx, tx, tenantId); err != nil {
			return err
		}
		return usecase.tenantRepository.Rename(ctx, tx, tenantId, name)
	})
}

func (usecase TenantUsecase) Merge(ctx context.Context, input TenantMergeInput) error {
	if err := usecase.enforceSecurity.Permission(models.TENANTS_MERGE); err != nil {
		return err
	}
	if input.TargetTenantId == uuid.Nil || len(input.SourceTenantIds) == 0 {
		return errors.Wrap(models.BadParameterError, "target tenant and source tenants are required")
	}
	seen := make(map[uuid.UUID]struct{}, len(input.SourceTenantIds))
	for _, sourceId := range input.SourceTenantIds {
		if sourceId == uuid.Nil || sourceId == input.TargetTenantId {
			return errors.Wrap(models.BadParameterError, "invalid source tenant")
		}
		if _, ok := seen[sourceId]; ok {
			return errors.Wrap(models.BadParameterError, "duplicate source tenant")
		}
		seen[sourceId] = struct{}{}
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.tenantRepository.LockAndValidateMerge(ctx, tx, input.TargetTenantId, input.SourceTenantIds); err != nil {
			return err
		}
		if err := usecase.grantRepository.ReassignTenantGrants(ctx, tx, input.TargetTenantId, input.SourceTenantIds); err != nil {
			return err
		}
		if err := usecase.organizationRepository.ReassignOrganizationsToTenant(ctx, tx, input.TargetTenantId, input.SourceTenantIds); err != nil {
			return err
		}
		if err := usecase.tenantRepository.SoftDelete(ctx, tx, input.SourceTenantIds); err != nil {
			return err
		}
		if input.NewName != nil {
			return usecase.tenantRepository.Rename(ctx, tx, input.TargetTenantId, *input.NewName)
		}
		return nil
	})
}
