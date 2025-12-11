package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type auditRepository interface {
	ListAuditEvents(ctx context.Context, exec repositories.Executor, pagination models.PaginationAndSorting, filters dto.AuditEventFilters) ([]models.AuditEvent, error)
}

type AuditUsecase struct {
	enforceSecurity security.EnforceSecurityAudit
	license         models.LicenseValidation
	executorFactory executor_factory.ExecutorFactory
	repository      auditRepository
}

func NewAuditUsecase(enforceSecurity security.EnforceSecurityAudit, executorFactory executor_factory.ExecutorFactory, license models.LicenseValidation, repository auditRepository) AuditUsecase {
	return AuditUsecase{
		enforceSecurity: enforceSecurity,
		executorFactory: executorFactory,
		license:         license,
		repository:      repository,
	}
}

func (uc AuditUsecase) ListAuditEvents(ctx context.Context, filters dto.AuditEventFilters) (models.Paginated[models.AuditEvent], error) {
	if uc.license.LicenseValidationCode != models.VALID {
		return models.Paginated[models.AuditEvent]{}, models.MissingLicenseEntitlementError
	}

	if err := uc.enforceSecurity.ReadAuditEvents(); err != nil {
		return models.Paginated[models.AuditEvent]{}, err
	}

	pagination := models.PaginationAndSorting{
		Limit:    filters.Limit + 1,
		OffsetId: filters.After,
	}

	events, err := uc.repository.ListAuditEvents(ctx, uc.executorFactory.NewExecutor(), pagination, filters)
	if err != nil {
		return models.Paginated[models.AuditEvent]{}, err
	}

	return models.Paginated[models.AuditEvent]{
		Items:       events[:min(filters.Limit, len(events))],
		HasNextPage: len(events) > filters.Limit,
	}, nil
}
