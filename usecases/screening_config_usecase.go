package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/pkg/errors"
)

type ScreeningConfigRepository interface {
	ListScreeningConfigs(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, useCache bool) ([]models.ScreeningConfig, error)
	GetScreeningConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId, id string) (models.ScreeningConfig, error)
	CreateScreeningConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, screeningConfig models.UpdateScreeningConfigInput) (models.ScreeningConfig, error)
	UpdateScreeningConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId, screeningId string, screeningConfig models.UpdateScreeningConfigInput) (models.ScreeningConfig, error)
	DeleteScreeningConfig(ctx context.Context, exec repositories.Executor, iterationId, configId string) error
}

func (uc ScreeningUsecase) CreateScreeningConfig(ctx context.Context, iterationId string,
	scCfg models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	scenarioAndIteration, err := uc.scenarioFetcher.FetchScenarioAndIteration(ctx,
		uc.executorFactory.NewExecutor(), iterationId)
	if err != nil {
		return models.ScreeningConfig{}, errors.Wrap(err,
			"could not find provided scenario iteration")
	}

	if err := uc.enforceSecurityScenario.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
		return models.ScreeningConfig{}, err
	}

	if scenarioAndIteration.Iteration.Version != nil {
		return models.ScreeningConfig{}, errors.Wrap(models.ErrScenarioIterationNotDraft,
			fmt.Sprintf("iteration %s is not a draft", scenarioAndIteration.Iteration.Id))
	}

	if scCfg.Query != nil {
		for field, v := range scCfg.Query {
			if v.Function != ast.FUNC_STRING_CONCAT {
				return models.ScreeningConfig{}, fmt.Errorf(
					"query field '%s' is not a StringConcat", field)
			}
		}
	}

	if scCfg.ForcedOutcome != nil &&
		!slices.Contains(models.ValidForcedOutcome, *scCfg.ForcedOutcome) {
		return models.ScreeningConfig{}, errors.Wrap(models.BadParameterError,
			"screening config: invalid forced outcome")
	}

	scc, err := uc.screeningConfigRepository.CreateScreeningConfig(ctx, uc.executorFactory.NewExecutor(),
		iterationId, scCfg)
	if err != nil {
		return models.ScreeningConfig{}, err
	}

	return scc, nil
}

func (uc ScreeningUsecase) UpdateScreeningConfig(ctx context.Context,
	iterationId, screeningId string,
	scCfg models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	scenarioAndIteration, err := uc.scenarioFetcher.FetchScenarioAndIteration(ctx,
		uc.executorFactory.NewExecutor(), iterationId)
	if err != nil {
		return models.ScreeningConfig{}, errors.Wrap(err,
			"could not find provided scenario iteration")
	}

	if err := uc.enforceSecurityScenario.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
		return models.ScreeningConfig{}, err
	}

	if scenarioAndIteration.Iteration.Version != nil {
		return models.ScreeningConfig{}, errors.Wrap(models.ErrScenarioIterationNotDraft,
			fmt.Sprintf("iteration %s is not a draft", scenarioAndIteration.Iteration.Id))
	}

	currentScc, err := uc.screeningConfigRepository.GetScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), iterationId, screeningId)
	if err != nil {
		return models.ScreeningConfig{}, err
	}

	if scCfg.EntityType != nil && *scCfg.EntityType != "Thing" {
		if scCfg.Preprocessing == nil {
			scCfg.Preprocessing = &currentScc.Preprocessing
		}

		scCfg.Preprocessing.UseNer = false
	}

	if scCfg.Query != nil {
		for field, v := range scCfg.Query {
			if v.Function != ast.FUNC_STRING_CONCAT {
				return models.ScreeningConfig{}, fmt.Errorf(
					"query filter '%s' must be a StringConcat", field)
			}
		}
	}

	if scCfg.ForcedOutcome != nil &&
		!slices.Contains(models.ValidForcedOutcome, *scCfg.ForcedOutcome) {
		return models.ScreeningConfig{}, errors.Wrap(models.BadParameterError,
			"screening config: invalid forced outcome")
	}

	scc, err := uc.screeningConfigRepository.UpdateScreeningConfig(ctx, uc.executorFactory.NewExecutor(),
		iterationId, screeningId, scCfg)
	if err != nil {
		return models.ScreeningConfig{}, err
	}

	return scc, nil
}

func (uc ScreeningUsecase) DeleteScreeningConfig(ctx context.Context, iterationId, configId string) error {
	scenarioAndIteration, err := uc.scenarioFetcher.FetchScenarioAndIteration(ctx,
		uc.executorFactory.NewExecutor(), iterationId)
	if err != nil {
		return errors.Wrap(err, "could not find provided scenario iteration")
	}

	if err := uc.enforceSecurityScenario.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
		return err
	}

	if scenarioAndIteration.Iteration.Version != nil {
		return errors.Wrap(models.ErrScenarioIterationNotDraft,
			fmt.Sprintf("iteration %s is not a draft", scenarioAndIteration.Iteration.Id))
	}

	return uc.screeningConfigRepository.DeleteScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), iterationId, configId)
}
