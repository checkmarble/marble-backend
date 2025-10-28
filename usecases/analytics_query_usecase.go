package usecases

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
)

type AnalyticsQueryUsecase struct {
	enforceSecurity security.EnforceSecurityScenario

	executorFactory  executor_factory.ExecutorFactory
	analyticsFactory executor_factory.AnalyticsExecutorFactory

	license            models.LicenseValidation
	scenarioRepository repositories.ScenarioUsecaseRepository
}

func (uc AnalyticsQueryUsecase) DecisionOutcomePerDay(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.DecisionOutcomePerDay, error) {
	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	subquery := squirrel.Select(fmt.Sprintf("time_bucket('1 day', created_at, '%s') as date, outcome, count() as decisions", filters.Timezone)).
		From(uc.analyticsFactory.BuildTarget("decisions", &scenario.TriggerObjectType)).
		Where("created_at between ? and ?", filters.Start, filters.End).
		GroupBy("date", "outcome")

	subquery, err = uc.analyticsFactory.ApplyFilters(subquery, scenario, filters)
	if err != nil {
		return nil, err
	}

	sql, args, err := subquery.ToSql()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		pivot (from (%s))
		on outcome in ('approve', 'decline', 'review', 'block_and_review')
		using coalesce(sum(decisions), 0)
		order by date`, sql)

	return repositories.AnalyticsRawScanStruct[analytics.DecisionOutcomePerDay](ctx, exec, query, args...)
}

func (uc AnalyticsQueryUsecase) DecisionsScoreDistribution(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.DecisionsScoreDistribution, error) {
	if !uc.license.Analytics {
		return []analytics.DecisionsScoreDistribution{}, nil
	}

	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	query := squirrel.Select("score, count() as decisions").
		From(uc.analyticsFactory.BuildTarget("decisions", &scenario.TriggerObjectType)).
		Where("created_at between ? and ?", filters.Start, filters.End).
		GroupBy("score").
		OrderBy("score")

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters)
	if err != nil {
		return nil, err
	}

	return repositories.AnalyticsScanStruct[analytics.DecisionsScoreDistribution](ctx, exec, query)
}

func (uc AnalyticsQueryUsecase) RuleHitTable(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.RuleHitTable, error) {
	if !uc.license.Analytics {
		return []analytics.RuleHitTable{}, nil
	}

	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	query := squirrel.
		Select(
			"rule_name",
			"count() filter (outcome = 'hit') as hit_count",
			"((count() filter (outcome = 'hit')) / count()) * 100 as hit_ratio",
			"count(distinct pivot_value) as pivot_count",
			"(count(distinct pivot_value) / count()) * 100 as pivot_ratio",
		).
		From(uc.analyticsFactory.BuildTarget("decision_rules", &scenario.TriggerObjectType)).
		Where("created_at between ? and ?", filters.Start, filters.End).
		Where("rule_name is not null").
		GroupBy("rule_id", "rule_name")

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters)
	if err != nil {
		return nil, err
	}

	return repositories.AnalyticsScanStruct[analytics.RuleHitTable](ctx, exec, query)
}

func (uc AnalyticsQueryUsecase) RuleVsDecisionOutcome(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.RuleVsDecisionOutcome, error) {
	if !uc.license.Analytics {
		return []analytics.RuleVsDecisionOutcome{}, nil
	}

	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	query := squirrel.
		Select(
			"rule_name",
			"d.outcome",
			"count() as decisions",
		).
		From(uc.analyticsFactory.BuildTarget("decision_rules", &scenario.TriggerObjectType, "dr")).
		InnerJoin(uc.analyticsFactory.BuildTarget("decisions", &scenario.TriggerObjectType, "d")+" on d.id = dr.decision_id").
		Where("d.created_at between ? and ?", filters.Start, filters.End).
		Where("rule_name is not null and dr.outcome = 'hit'").
		GroupBy("stable_rule_id", "rule_name", "d.outcome")

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters, "dr")
	if err != nil {
		return nil, err
	}

	return repositories.AnalyticsScanStruct[analytics.RuleVsDecisionOutcome](ctx, exec, query)
}

func (uc AnalyticsQueryUsecase) RuleCoOccurenceMatrix(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.RuleCoOccurence, error) {
	if !uc.license.Analytics {
		return []analytics.RuleCoOccurence{}, nil
	}

	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	query := squirrel.
		Select(
			"t1.rule_id as rule_x",
			"any_value(t1.rule_name) as rule_x_name",
			"t2.rule_id as rule_y",
			"any_value(t2.rule_name) as rule_y_name",
			"count() as decisions",
		).
		From(uc.analyticsFactory.BuildTarget("decision_rules", &scenario.TriggerObjectType, "t1")).
		Join(uc.analyticsFactory.BuildTarget("decision_rules", &scenario.TriggerObjectType, "t2")+" on t1.decision_id = t2.decision_id").
		Where("t1.created_at between ? and ?", filters.Start, filters.End).
		Where("t1.rule_id >= t2.rule_id").
		Where("t1.outcome = 'hit' and t2.outcome = 'hit'").
		GroupBy("rule_x", "rule_y")

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters, "t1")
	if err != nil {
		return nil, err
	}

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters, "t2")
	if err != nil {
		return nil, err
	}

	return repositories.AnalyticsScanStruct[analytics.RuleCoOccurence](ctx, exec, query)
}

func (uc AnalyticsQueryUsecase) ScreeningHits(ctx context.Context, filters dto.AnalyticsQueryFilters) ([]analytics.ScreeningHits, error) {
	if !uc.license.Analytics {
		return []analytics.ScreeningHits{}, nil
	}

	scenario, exec, err := uc.getExecutor(ctx, filters.ScenarioId)
	if err != nil {
		return nil, err
	}

	query := squirrel.
		Select(
			"screening_config_id",
			"any_value(screening_name) as screening_name",
			"count() as execs",
			"count() filter (matches > 0) as hits",
			"coalesce((hits / execs) * 100, 0.0) as hit_ratio",
			"coalesce(avg(matches) filter (matches > 0), 0) as avg_hits",
		).
		From(uc.analyticsFactory.BuildTarget("screenings", &scenario.TriggerObjectType)).
		Where("created_at between ? and ?", filters.Start, filters.End).
		GroupBy("screening_config_id")

	query, err = uc.analyticsFactory.ApplyFilters(query, scenario, filters)
	if err != nil {
		return nil, err
	}

	return repositories.AnalyticsScanStruct[analytics.ScreeningHits](ctx, exec, query)
}

func (uc AnalyticsQueryUsecase) getExecutor(ctx context.Context, scenarioId uuid.UUID) (models.Scenario, repositories.AnalyticsExecutor, error) {
	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, uc.executorFactory.NewExecutor(), scenarioId.String())
	if err != nil {
		return models.Scenario{}, nil, err
	}
	if err := uc.enforceSecurity.ReadScenario(scenario); err != nil {
		return models.Scenario{}, nil, err
	}

	exec, err := uc.analyticsFactory.GetExecutor(ctx)
	if err != nil {
		return models.Scenario{}, nil, err
	}

	return scenario, exec, nil
}
