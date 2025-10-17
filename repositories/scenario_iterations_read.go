package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/Masterminds/squirrel"
)

var scenarioIterationCache = expirable.NewLRU[string, models.ScenarioIteration](50, nil, utils.GlobalCacheDuration())

func (repo *MarbleDbRepository) GetScenarioIteration(
	ctx context.Context,
	exec Executor,
	scenarioIterationId string,
	useCache bool,
) (models.ScenarioIteration, error) {
	if useCache && repo.withCache {
		if iteration, ok := scenarioIterationCache.Get(scenarioIterationId); ok {
			return iteration, nil
		}
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	iteration, err := SqlToModel(
		ctx,
		exec,
		selectScenarioIterations().Where(squirrel.Eq{"si.id": scenarioIterationId}),
		dbmodels.AdaptScenarioIterationWithRules,
	)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	scenarioIterationCache.Add(scenarioIterationId, iteration)

	return iteration, nil
}

func (repo *MarbleDbRepository) ListScenarioIterations(
	ctx context.Context,
	exec Executor,
	organizationId string,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := selectScenarioIterations().Where(squirrel.Eq{"si.org_id": organizationId})
	if filters.ScenarioId != nil {
		sql = sql.Where(squirrel.Eq{"si.scenario_id": *filters.ScenarioId})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		sql,
		dbmodels.AdaptScenarioIterationWithRules,
	)
}

func selectScenarioIterations() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("si", dbmodels.SelectScenarioIterationColumn)...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY sir.created_at) FILTER (WHERE sir.id IS NOT NULL) as rules",
				strings.Join(columnsNames("sir", dbmodels.SelectRulesColumn), ","),
			),
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS si").
		LeftJoin(dbmodels.TABLE_RULES + " AS sir ON sir.scenario_iteration_id = si.id").
		GroupBy("si.id").
		OrderBy("si.created_at DESC")
}
