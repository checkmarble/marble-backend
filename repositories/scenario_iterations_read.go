package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func (repository *MarbleDbRepository) GetScenarioIteration(
	ctx context.Context,
	exec Executor,
	scenarioIterationId string,
) (models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScenarioIteration{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectScenarioIterations().Where(squirrel.Eq{"si.id": scenarioIterationId}),
		dbmodels.AdaptScenarioIterationWithRules,
	)
}

func (repository *MarbleDbRepository) ListScenarioIterations(
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
