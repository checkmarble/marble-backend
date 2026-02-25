package scoring

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

const MAX_SCORE_SCALE = 6

type ScoringSettingsUsecase struct {
	enforceSecurity    security.EnforceSecurityScoring
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	repository         scoringRepository
}

func NewScoringSettingsUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository scoringRepository,
) ScoringSettingsUsecase {
	return ScoringSettingsUsecase{
		enforceSecurity:    enforceSecurity,
		executorFactory:    executorFactory,
		transactionFactory: transactionFactory,
		repository:         repository,
	}
}

func (uc ScoringSettingsUsecase) GetSettings(ctx context.Context) (models.ScoringSettings, error) {
	settings, err := uc.repository.GetScoringSettings(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return models.ScoringSettings{}, err
	}
	if settings == nil {
		return models.ScoringSettings{}, errors.Wrap(models.NotFoundError, "no scoring settings found")
	}

	return *settings, nil
}

func (uc ScoringSettingsUsecase) UpdateSettings(ctx context.Context, settings models.ScoringSettings) (models.ScoringSettings, error) {
	settings.OrgId = uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.UpdateSettings(settings.OrgId); err != nil {
		return models.ScoringSettings{}, err
	}

	if settings.MaxScore < 1 || settings.MaxScore > MAX_SCORE_SCALE {
		return models.ScoringSettings{}, errors.Wrapf(models.BadParameterError, "maximum score outside of allowed bounds (1-%d)", MAX_SCORE_SCALE)
	}

	return uc.repository.UpdateScoringSettings(ctx, uc.executorFactory.NewExecutor(), settings)
}
