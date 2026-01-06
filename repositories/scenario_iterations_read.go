package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
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
	organizationId uuid.UUID,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := selectScenarioIterations().
		Where(squirrel.Eq{
			"si.org_id":      organizationId,
			"si.scenario_id": filters.ScenarioId,
		})

	return SqlToListOfModels(
		ctx,
		exec,
		sql,
		dbmodels.AdaptScenarioIterationWithRules,
	)
}

func (repo *MarbleDbRepository) ListScenarioIterationsMetadata(
	ctx context.Context,
	exec Executor,
	organizationId uuid.UUID,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIterationMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectScenarioIterationMetadataColumn...).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS).
		Where(squirrel.Eq{
			"org_id":      organizationId,
			"scenario_id": filters.ScenarioId,
		}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		ctx,
		exec,
		sql,
		dbmodels.AdaptScenarioIterationMetadata,
	)
}

func selectScenarioIterations() squirrel.SelectBuilder {
	query := NewQueryBuilder().
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

	return query
}

func (repo *MarbleDbRepository) ListAllRulesAndScreenings(
	ctx context.Context,
	exec Executor,
	organizationId string,
) ([]models.RulesAndScreenings, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	rules := NewQueryBuilder().
		Select(
			"si.id", "si.scenario_id", "si.version", "si.trigger_condition_ast_expression as trigger_ast",
			"sir.id as rule_id",
			"sir.formula_ast_expression as rule_ast",
			"null as screening_trigger_ast", "null as screening_counterparty_ast", "null as screening_ast",
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS+" si").
		LeftJoin(dbmodels.TABLE_RULES+" sir on si.id = sir.scenario_iteration_id").
		Where("si.org_id = ? and sir.id is not null", organizationId)

	screenings := NewQueryBuilder().
		Select(
			"si.id", "si.scenario_id", "si.version", "si.trigger_condition_ast_expression as trigger_ast",
			"sc.id as rule_id",
			"null as rule_ast",
			"sc.trigger_rule as screening_trigger_ast", "sc.counterparty_id_expression as screening_counterparty_ast", "sc.query as screening_ast",
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS+" si").
		LeftJoin(dbmodels.TABLE_SCREENING_CONFIGS+" sc on si.id = sc.scenario_iteration_id").
		Where("si.org_id = ? and sc.id is not null", organizationId)

	sql := rules.SuffixExpr(screenings.Prefix(" UNION ALL "))

	return SqlToListOfModels(
		ctx,
		exec,
		sql,
		dbmodels.AdaptRulesAndScreenings,
	)
}
