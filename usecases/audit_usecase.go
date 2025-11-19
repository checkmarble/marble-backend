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
	ListAuditEvents(ctx context.Context, exec repositories.Executor, filters dto.AuditEventFilters) ([]models.AuditEvent, error)
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

func (uc AuditUsecase) ListAuditEvents(ctx context.Context, filters dto.AuditEventFilters) ([]models.AuditEvent, error) {
	if uc.license.LicenseValidationCode != models.VALID {
		return nil, models.MissingLicenseEntitlementError
	}

	if err := uc.enforceSecurity.ReadAuditEvents(); err != nil {
		return nil, err
	}

	return uc.repository.ListAuditEvents(ctx, uc.executorFactory.NewExecutor(), filters)
}
