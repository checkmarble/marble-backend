package repositories

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/pure_utils"
	"marble/marble-backend/repositories/dbmodels"
	"strings"

	"github.com/Masterminds/squirrel"
)

type ScenarioIterationReadRepository interface {
	GetScenarioIteration(tx Transaction, scenarioIterationID string) (
		models.ScenarioIteration, error,
	)
	ListScenarioIterations(tx Transaction, organizationId string, filters models.GetScenarioIterationFilters) (
		[]models.ScenarioIteration, error,
	)
}

type ScenarioIterationReadRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repository *ScenarioIterationReadRepositoryPostgresql) GetScenarioIteration(
	tx Transaction,
	scenarioIterationID string,
) (models.ScenarioIteration, error) {
	pgTx := repository.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModelAdapterWithErr(
		pgTx,
		selectScenarioIterations().Where(squirrel.Eq{"si.id": scenarioIterationID}),
		dbmodels.AdaptScenarioIterationWithRules,
	)
}

func (repository *ScenarioIterationReadRepositoryPostgresql) ListScenarioIterations(
	tx Transaction,
	organizationId string,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	pgTx := repository.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	sql := selectScenarioIterations().Where(squirrel.Eq{"si.org_id": organizationId})
	if filters.ScenarioID != nil {
		sql = sql.Where(squirrel.Eq{"si.scenario_id": *filters.ScenarioID})
	}

	return SqlToListOfModelsAdapterWithErr(
		pgTx,
		sql,
		dbmodels.AdaptScenarioIterationWithRules,
	)

}

func selectScenarioIterations() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(pure_utils.WithPrefix(dbmodels.SelectScenarioIterationColumn, "si")...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY sir.created_at) FILTER (WHERE sir.id IS NOT NULL) as rules",
				strings.Join(pure_utils.WithPrefix(dbmodels.SelectRulesColumn, "sir"), ","),
			),
		).
		From(dbmodels.TABLE_SCENARIO_ITERATIONS + " AS si").
		LeftJoin(dbmodels.TABLE_RULES + " AS sir ON sir.scenario_iteration_id = si.id").
		GroupBy("si.id").
		OrderBy("si.created_at DESC")
}
