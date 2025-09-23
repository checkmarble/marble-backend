package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type AnalyticsMetadataUsecase struct {
	enforceSecurity security.EnforceSecurityScenario

	executorFactory  executor_factory.ExecutorFactory
	analyticsFactory executor_factory.AnalyticsExecutorFactory

	scenarioRepository repositories.ScenarioUsecaseRepository
}

func (uc AnalyticsMetadataUsecase) GetAvailableFilters(ctx context.Context, req dto.AnalyticsAvailableFiltersRequest) ([]models.AnalyticsFilter, error) {
	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, uc.executorFactory.NewExecutor(), req.ScenarioId.String())
	if err != nil {
		return nil, err
	}
	if err := uc.enforceSecurity.ReadScenario(scenario); err != nil {
		return nil, err
	}

	exec, err := uc.analyticsFactory.GetExecutor(ctx)
	if err != nil {
		return nil, err
	}

	inner := squirrel.Select("*").From(uc.analyticsFactory.BuildTarget("decisions", &scenario.TriggerObjectType))
	inner = uc.analyticsFactory.BuildPushdownFilter(inner, uc.enforceSecurity.OrgId(), req.Start, req.End, scenario.TriggerObjectType)

	innerSql, innerArgs, _ := inner.ToSql()

	rows, err := exec.QueryContext(ctx, fmt.Sprintf(
		`
			select column_name, column_type
			from (describe select * from %s) o
			inner join (
			  select distinct name from (
			    unpivot(%s)
			    on columns('^(tr|ex)_')::varchar
			  )
			) i
			on i.name = o.column_name;
		`,
		uc.analyticsFactory.BuildTarget("decisions", &scenario.TriggerObjectType),
		innerSql,
	), innerArgs...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tmp models.AnalyticsFilter

	filters := make([]models.AnalyticsFilter, 0)

	for rows.Next() {
		if err := rows.Scan(&tmp.Name, &tmp.Type); err != nil {
			return nil, err
		}

		if strings.HasPrefix(tmp.Name, "tr_") || strings.HasPrefix(tmp.Name, "ex_") {
			filters = append(filters, tmp)
		}
	}

	return filters, rows.Err()
}
