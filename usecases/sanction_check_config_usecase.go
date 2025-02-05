package usecases

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/pkg/errors"
)

type SanctionCheckConfigRepository interface {
	GetSanctionCheckConfig(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (*models.SanctionCheckConfig, error)
	UpdateSanctionCheckConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, sanctionCheckConfig models.UpdateSanctionCheckConfigInput) (models.SanctionCheckConfig, error)
}

func (uc SanctionCheckUsecase) ConfigureSanctionCheck(ctx context.Context,
	iterationId string,
	scCfg models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	if scCfg.Query != nil {
		if scCfg.Query.Name.Function != ast.FUNC_STRING_CONCAT {
			return models.SanctionCheckConfig{}, errors.New(
				"query name filter must be a StringConcat")
		}
	}

	if scCfg.Outcome.ForceOutcome != nil &&
		!slices.Contains(models.ValidForcedOutcome, *scCfg.Outcome.ForceOutcome) {
		return models.SanctionCheckConfig{}, errors.Wrap(models.BadParameterError,
			"sanction check config: invalid forced outcome")
	}

	scc, err := uc.sanctionCheckConfigRepository.UpdateSanctionCheckConfig(ctx, uc.executorFactory.NewExecutor(),
		iterationId, scCfg)
	if err != nil {
		return models.SanctionCheckConfig{}, err
	}

	return scc, nil
}
