package scoring

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

const MAX_RISK_LEVEL_SCALE = 6

type ScoringSettingsUsecase struct {
	enforceSecurity    security.EnforceSecurityScoring
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	repository         ScoringRepository
}

func NewScoringSettingsUsecase(
	enforceSecurity security.EnforceSecurityScoring,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	repository ScoringRepository,
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

	if settings.MaxRiskLevel < 1 || settings.MaxRiskLevel > MAX_RISK_LEVEL_SCALE {
		return models.ScoringSettings{}, errors.Wrapf(models.BadParameterError, "maximum risk level outside of allowed bounds (1-%d)", MAX_RISK_LEVEL_SCALE)
	}

	exec := uc.executorFactory.NewExecutor()

	existingSettings, err := uc.repository.GetScoringSettings(ctx, exec, settings.OrgId)
	if err != nil {
		return models.ScoringSettings{}, err
	}

	// Some settings cannot be changed after a ruleset is created.
	if existingSettings != nil {
		rulesets, err := uc.repository.ListScoringRulesets(ctx, exec, settings.OrgId)
		if err != nil {
			return models.ScoringSettings{}, err
		}

		if len(rulesets) > 0 && existingSettings.MaxRiskLevel != settings.MaxRiskLevel {
			return models.ScoringSettings{}, errors.Wrap(models.BadParameterError, "cannot change maximum risk level after having created rulesets")
		}
	}

	return uc.repository.UpdateScoringSettings(ctx, exec, settings)
}
