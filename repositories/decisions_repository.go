package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionRepository interface {
	DecisionWithRuleExecutionsById(
		ctx context.Context,
		exec Executor,
		decisionId string,
	) (models.DecisionWithRuleExecutions, error)
	DecisionsWithRuleExecutionsByIds(
		ctx context.Context,
		exec Executor,
		decisionIds []string,
	) ([]models.DecisionWithRuleExecutions, error)
	DecisionsById(ctx context.Context, exec Executor, decisionIds []string) ([]models.Decision, error)
	DecisionsByCaseId(
		ctx context.Context,
		exec Executor,
		organizationId, caseId string,
	) ([]models.DecisionWithRuleExecutions, error)
	DecisionsByCaseIdFromCursor(
		ctx context.Context,
		exec Executor,
		req models.CaseDecisionsRequest) ([]models.DecisionWithRuleExecutions, bool, error)
	DecisionsByObjectId(ctx context.Context, exec Executor, organizationId string, objectId string) ([]models.DecisionCore, error)
	DecisionsOfScheduledExecution(
		ctx context.Context,
		exec Executor,
		organizationId string,
		scheduledExecutionId string,
	) (<-chan models.DecisionWithRuleExecutions, <-chan error)
	StoreDecision(
		ctx context.Context,
		exec Executor,
		decision models.DecisionWithRuleExecutions,
		organizationId string,
		newDecisionId string) error
	DecisionsOfOrganizationWithRank(
		ctx context.Context,
		exec Executor,
		organizationId string,
		paginationAndSorting models.PaginationAndSorting,
		filters models.DecisionFilters,
	) ([]models.DecisionWithRank, error)
	DecisionsOfOrganization(
		ctx context.Context,
		exec Executor,
		organizationId string,
		paginationAndSorting models.PaginationAndSorting,
		filters models.DecisionFilters,
	) ([]models.Decision, error)
	UpdateDecisionCaseId(ctx context.Context, exec Executor, decisionsIds []string, caseId string) error
	ReviewDecision(ctx context.Context, exec Executor, decisionId string, reviewStatus string) error
	DecisionPivotValuesByCase(ctx context.Context, exec Executor, caseId string) ([]models.PivotDataWithCount, error)
}

// the size of the batch is chosen without any benchmark
const decisionRulesBatchSize = 1000

func (repo *MarbleDbRepository) DecisionWithRuleExecutionsById(ctx context.Context, exec Executor,
	decisionId string,
) (models.DecisionWithRuleExecutions, error) {
	decisions, err := repo.DecisionsWithRuleExecutionsByIds(ctx, exec, []string{decisionId})
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	if len(decisions) == 0 {
		return models.DecisionWithRuleExecutions{},
			errors.Wrap(models.NotFoundError, "Decision not found")
	}

	return decisions[0], nil
}

func (repo *MarbleDbRepository) DecisionsByOutcomeAndScore(
	ctx context.Context,
	exec Executor,
	organizationId string,
	scenarioId string,
	scenarioLiveIterationId string,
	begin, end time.Time,
) ([]models.DecisionsByVersionByOutcome, error) {
	decisionQuery := squirrel.StatementBuilder.
		Select("outcome, scenario_version, score").
		From(dbmodels.TABLE_DECISIONS).
		Where(squirrel.GtOrEq{
			"created_at": begin,
		}).
		Where(squirrel.LtOrEq{
			"created_at": end,
		}).
		Where(squirrel.Eq{
			"org_id":      organizationId,
			"scenario_id": scenarioId,
		})
	phantomDecisionQuery := squirrel.StatementBuilder.
		Select("outcome, scenario_version, score").
		From(dbmodels.TABLE_PHANTOM_DECISIONS).
		Where(squirrel.GtOrEq{
			"created_at": begin,
		}).
		Where(squirrel.LtOrEq{
			"created_at": end,
		}).
		Where(squirrel.Eq{
			"org_id":      organizationId,
			"scenario_id": scenarioId,
		})
	query, err := WithUnionAll(decisionQuery, phantomDecisionQuery)
	if err != nil {
		return nil, err
	}
	finalQuery := NewQueryBuilder().
		Select("q.scenario_version, q.outcome, q.score, Count(q.outcome) as total").
		FromSelect(query, "q").
		GroupBy("scenario_version, outcome, score").
		PlaceholderFormat(squirrel.Dollar)

	out, err := SqlToListOfRow(ctx,
		exec,
		finalQuery,
		func(row pgx.CollectableRow) (models.DecisionsByVersionByOutcome, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DbDecisionsByOutcome](row)
			if err != nil {
				return models.DecisionsByVersionByOutcome{}, err
			}
			return dbmodels.AdaptDecisionByOutcome(db), nil
		})
	if err != nil {
		return nil, err
	}

	// return at least one count object with 0 count by default, because the watermark on it is needed for the summary calculation.
	// See the usage in the caller test run summary job.
	if len(out) == 0 {
		si, err := repo.GetScenarioIteration(ctx, exec, scenarioLiveIterationId)
		if err != nil {
			return nil, err
		}
		return []models.DecisionsByVersionByOutcome{
			{
				Outcome: "approve",
				Version: fmt.Sprintf("%d", *si.Version),
				Score:   0,
			},
		}, nil
	}

	return out, nil
}

func (repo *MarbleDbRepository) DecisionsWithRuleExecutionsByIds(
	ctx context.Context,
	exec Executor,
	decisionIds []string,
) ([]models.DecisionWithRuleExecutions, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	rules, err := repo.rulesOfDecisions(ctx, exec, decisionIds)
	if err != nil {
		return nil, err
	}

	return SqlToListOfRow(
		ctx,
		exec,
		selectJoinDecisionAndCase().
			Where(squirrel.Eq{"d.id": decisionIds}),
		func(row pgx.CollectableRow) (models.DecisionWithRuleExecutions, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DbJoinDecisionAndCase](row)
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}

			var decisionCase *models.Case
			if db.DbDecision.CaseId != nil {
				decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
				if err != nil {
					return models.DecisionWithRuleExecutions{}, err
				}
				decisionCase = &decisionCaseValue
			}

			return dbmodels.AdaptDecisionWithRuleExecutions(
				db.DbDecision,
				rules[db.DbDecision.Id],
				decisionCase,
			), nil
		},
	)
}

func (repo *MarbleDbRepository) DecisionsById(ctx context.Context, exec Executor, decisionIds []string) ([]models.Decision, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectJoinDecisionAndCase().Where(squirrel.Eq{"d.id": decisionIds})

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbJoinDecisionAndCase](row)
		if err != nil {
			return models.Decision{}, err
		}

		var decisionCase *models.Case
		if db.DbDecision.CaseId != nil {
			decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
			if err != nil {
				return models.Decision{}, err
			}
			decisionCase = &decisionCaseValue
		}
		return dbmodels.AdaptDecision(db.DbDecision, decisionCase), nil
	})
}

func (repo *MarbleDbRepository) DecisionsByCaseId(
	ctx context.Context,
	exec Executor,
	organizationId string,
	caseId string,
) ([]models.DecisionWithRuleExecutions, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectDecisions().
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"case_id": caseId}).
		OrderBy("created_at DESC")

	decisionsChan, errChan := repo.channelOfDecisions(ctx, exec, query)

	decisions := ChanToSlice(decisionsChan)
	err := <-errChan

	return decisions, err
}

func (repo *MarbleDbRepository) DecisionsByCaseIdFromCursor(
	ctx context.Context,
	exec Executor,
	req models.CaseDecisionsRequest,
) ([]models.DecisionWithRuleExecutions, bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, false, err
	}

	query := selectDecisions().
		Where(squirrel.Eq{"org_id": req.OrgId}).
		Where(squirrel.Eq{"case_id": req.CaseId}).
		Limit(uint64(req.Limit) + 1).
		OrderBy("created_at DESC, id DESC")

	if req.CursorId != "" {
		cursorDecision, err := repo.DecisionsById(ctx, exec, []string{req.CursorId})
		if err != nil {
			return nil, false, err
		}
		if len(cursorDecision) == 0 {
			return nil, false, errors.Wrap(models.NotFoundError, "could not find decision for cursor")
		}

		query = query.Where(squirrel.Expr("(created_at, id) < (?, ?)",
			cursorDecision[0].CreatedAt, req.CursorId))
	}

	decisionsChan, errChan := repo.channelOfDecisions(ctx, exec, query)

	decisions := ChanToSlice(decisionsChan)

	if err := <-errChan; err != nil {
		return nil, false, err
	}

	if len(decisions) == 0 {
		return []models.DecisionWithRuleExecutions{}, false, nil
	}

	hasMore := len(decisions) > req.Limit

	return decisions[:min(len(decisions), req.Limit)], hasMore, nil
}

func (repo *MarbleDbRepository) DecisionsByObjectId(
	ctx context.Context,
	exec Executor,
	organizationId string,
	objectId string,
) ([]models.DecisionCore, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectDecisions().
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"trigger_object->>'object_id'": objectId}).
		OrderBy("created_at DESC")

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.DecisionCore, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbDecision](row)
		if err != nil {
			return models.DecisionCore{}, err
		}

		return dbmodels.AdaptDecisionCore(db), nil
	})
}

func (repo *MarbleDbRepository) DecisionsOfOrganization(
	ctx context.Context,
	exec Executor,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters models.DecisionFilters,
) ([]models.Decision, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if pagination.Sorting != models.SortingFieldCreatedAt {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", pagination.Sorting)
	}

	orderCond := orderConditionForDecisions(pagination)

	filteredSubquery := selectDecisionsWithFilters(organizationId, orderCond, filters, false)

	paginatedQuery := NewQueryBuilder().
		Select(columnsNames("s", dbmodels.SelectDecisionColumn)...).
		FromSelect(filteredSubquery, "s").
		Limit(uint64(pagination.Limit))

	var offsetDecision models.DecisionWithRuleExecutions
	if pagination.OffsetId != "" {
		var err error
		offsetDecision, err = repo.DecisionWithRuleExecutionsById(ctx, exec, pagination.OffsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offsetId")
		} else if err != nil {
			return nil, errors.Wrap(err,
				"failed to fetch decision corresponding to the provided offsetId")
		}
	}

	paginatedQuery, err := applyDecisionPaginationFilters(paginatedQuery, pagination, offsetDecision.Decision)
	if err != nil {
		return nil, err
	}
	query := selectDecisionsWithJoinedFields(paginatedQuery, orderCond)

	decision, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbJoinDecisionAndCase](row)
		if err != nil {
			return models.Decision{}, err
		}

		var decisionCase *models.Case
		if db.DbDecision.CaseId != nil {
			decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
			if err != nil {
				return models.Decision{}, err
			}
			decisionCase = &decisionCaseValue
		}
		return dbmodels.AdaptDecision(db.DbDecision, decisionCase), nil
	})
	if err != nil {
		return nil, err
	}
	return decision, nil
}

func (repo *MarbleDbRepository) DecisionsOfOrganizationWithRank(
	ctx context.Context,
	exec Executor,
	organizationId string,
	pagination models.PaginationAndSorting,
	filters models.DecisionFilters,
) ([]models.DecisionWithRank, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if pagination.Sorting != models.SortingFieldCreatedAt {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", pagination.Sorting)
	}

	orderCond := orderConditionForDecisions(pagination)

	filteredSubquery := selectDecisionsWithFilters(organizationId, orderCond, filters, true)

	columns := columnsNames("s", append(dbmodels.SelectDecisionColumn, "rank_number"))
	paginatedQuery := NewQueryBuilder().
		Select(columns...).
		FromSelect(filteredSubquery, "s").
		Limit(uint64(pagination.Limit))

	var offsetDecision models.DecisionWithRuleExecutions
	if pagination.OffsetId != "" {
		var err error
		offsetDecision, err = repo.DecisionWithRuleExecutionsById(ctx, exec, pagination.OffsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return []models.DecisionWithRank{}, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offsetId")
		} else if err != nil {
			return []models.DecisionWithRank{}, errors.Wrap(err,
				"failed to fetch decision corresponding to the provided offsetId")
		}
	}

	paginatedQuery, err := applyDecisionPaginationFilters(paginatedQuery, pagination, offsetDecision.Decision)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}
	query := selectDecisionsWithJoinedFields(paginatedQuery, orderCond).
		Column("rank_number")

	decision, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.DecisionWithRank, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBPaginatedDecisions](row)
		if err != nil {
			return models.DecisionWithRank{}, err
		}

		var decisionCase *models.Case
		if db.DbDecision.CaseId != nil {
			decisionCaseValue, err := dbmodels.AdaptCase(db.DBCase)
			if err != nil {
				return models.DecisionWithRank{}, err
			}
			decisionCase = &decisionCaseValue
		}
		return dbmodels.AdaptDecisionWithRank(db.DbDecision, decisionCase, db.RankNumber), nil
	})
	if err != nil {
		return []models.DecisionWithRank{}, err
	}
	return decision, nil
}

func orderConditionForDecisions(p models.PaginationAndSorting) string {
	return fmt.Sprintf("d.%s %s, d.id %s", p.Sorting, p.Order, p.Order)
}

func selectDecisionsWithFilters(
	organizationId string,
	orderCond string,
	filters models.DecisionFilters,
	withRank bool,
) squirrel.SelectBuilder {
	query := NewQueryBuilder().
		Select(columnsNames("d", dbmodels.SelectDecisionColumn)...).
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		Where(squirrel.Eq{"d.org_id": organizationId}).
		OrderBy(orderCond)

	if withRank {
		query = query.Column(fmt.Sprintf("RANK() OVER (ORDER BY %s) as rank_number", orderCond))
	}

	if len(filters.ScenarioIds) > 0 {
		query = query.Where(squirrel.Eq{"d.scenario_id": filters.ScenarioIds})
	}
	if len(filters.Outcomes) > 0 {
		query = query.Where(squirrel.Eq{"d.outcome": filters.Outcomes})
	}
	if len(filters.TriggerObjects) > 0 {
		query = query.Where(squirrel.Eq{"d.trigger_object_type": filters.TriggerObjects})
	}
	if !filters.StartDate.IsZero() {
		query = query.Where(squirrel.GtOrEq{"d.created_at": filters.StartDate})
	}
	if !filters.EndDate.IsZero() {
		query = query.Where(squirrel.LtOrEq{"d.created_at": filters.EndDate})
	}
	if filters.HasCase != nil && *filters.HasCase {
		query = query.Where(squirrel.NotEq{"d.case_id": nil})
	}
	if filters.HasCase != nil && !*filters.HasCase {
		query = query.Where(squirrel.Eq{"d.case_id": nil})
	}
	if len(filters.CaseIds) > 0 {
		query = query.Where(squirrel.Eq{"d.case_id": filters.CaseIds})
	}
	if len(filters.ReviewStatuses) > 0 {
		query = query.Where(squirrel.Eq{"d.review_status": filters.ReviewStatuses})
	}
	if len(filters.ScheduledExecutionIds) > 0 {
		query = query.Where(squirrel.Eq{"d.scheduled_execution_id": filters.ScheduledExecutionIds})
	}
	if filters.PivotValue != nil {
		query = query.Where(squirrel.Eq{"d.pivot_value": *filters.PivotValue})
	}

	// only if we want to filter by case inbox id, join the cases table
	if len(filters.CaseInboxIds) > 0 {
		query = query.
			Join(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
			Where(squirrel.Eq{"c.inbox_id": filters.CaseInboxIds})
	}

	return query
}

func applyDecisionPaginationFilters(query squirrel.SelectBuilder, p models.PaginationAndSorting, offset models.Decision) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	var offsetValue any
	switch p.Sorting {
	case models.DecisionSortingCreatedAt:
		offsetValue = offset.CreatedAt
	default:
		// only ordering and pagination by created_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	args := []any{offsetValue, offsetValue, p.OffsetId}
	if p.Order == models.SortingOrderDesc {
		query = query.Where(fmt.Sprintf("%s < ? OR (%s = ? AND id < ?)", p.Sorting, p.Sorting), args...)
	} else {
		query = query.Where(fmt.Sprintf("%s > ? OR (%s = ? AND id > ?)", p.Sorting, p.Sorting), args...)
	}

	return query, nil
}

func selectDecisionsWithJoinedFields(query squirrel.SelectBuilder, orderCond string) squirrel.SelectBuilder {
	columns := columnsNames("d", dbmodels.SelectDecisionColumn)
	columns = append(columns, columnsNames("c", dbmodels.SelectCaseColumn)...)
	return squirrel.
		Select(columns...).
		FromSelect(query, "d").
		LeftJoin(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
		OrderBy(orderCond).
		PlaceholderFormat(squirrel.Dollar)
}

func (repo *MarbleDbRepository) DecisionsOfScheduledExecution(
	ctx context.Context,
	exec Executor,
	organizationId string,
	scheduledExecutionId string,
) (<-chan models.DecisionWithRuleExecutions, <-chan error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		valChannel := make(chan models.DecisionWithRuleExecutions)
		errChannel := make(chan error)
		errChannel <- err
		close(valChannel)
		close(errChannel)
		return valChannel, errChannel
	}

	return repo.channelOfDecisions(
		ctx,
		exec,
		selectDecisions().
			Where(squirrel.Eq{"org_id": organizationId}).
			Where(squirrel.Eq{"scheduled_execution_id": scheduledExecutionId}).
			OrderBy("created_at DESC"),
	)
}

// Nb: Beware that the decision usecase sends a complete decision object, and only reads it back if a case has been added
// => do not add values directly at the repository or db level, or adjust the usecase logic accordingly.
func (repo *MarbleDbRepository) StoreDecision(
	ctx context.Context,
	exec Executor,
	decision models.DecisionWithRuleExecutions,
	organizationId string,
	newDecisionId string,
) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionRepository.StoreDecision.store_decision",
		trace.WithAttributes(attribute.String("decision_id", newDecisionId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
	defer span.End()
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_DECISIONS).
			Columns(
				"id",
				"org_id",
				"created_at",
				"outcome",
				"pivot_id",
				"pivot_value",
				"review_status",
				"scenario_id",
				"scenario_iteration_id",
				"scenario_name",
				"scenario_description",
				"scenario_version",
				"score",
				"trigger_object",
				"trigger_object_type",
				"scheduled_execution_id",
			).
			Values(
				newDecisionId,
				organizationId,
				decision.CreatedAt,
				decision.Outcome.String(),
				decision.PivotId,
				decision.PivotValue,
				decision.ReviewStatus,
				decision.ScenarioId,
				decision.ScenarioIterationId,
				decision.ScenarioName,
				decision.ScenarioDescription,
				decision.ScenarioVersion,
				decision.Score,
				decision.ClientObject.Data,
				decision.ClientObject.TableName,
				decision.ScheduledExecutionId,
			),
	)
	if err != nil {
		return err
	}

	if len(decision.RuleExecutions) == 0 {
		return nil
	}

	ctx, span = tracer.Start(
		ctx,
		"DecisionRepository.StoreDecision.store_decision_rules",
		trace.WithAttributes(attribute.String("decision_id", newDecisionId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
	defer span.End()
	builderForRules := NewQueryBuilder().
		Insert(dbmodels.TABLE_DECISION_RULES).
		Columns(
			"id",
			"org_id",
			"decision_id",
			"score_modifier",
			"result",
			"error_code",
			"rule_id",
			"rule_evaluation",
			"outcome",
		)

	for _, ruleExecution := range decision.RuleExecutions {
		serializedRuleEvaluation, err := dbmodels.SerializeNodeEvaluationDto(ruleExecution.Evaluation)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("rule(%s):", ruleExecution.Rule.Id))
		}

		builderForRules = builderForRules.
			Values(
				uuid.Must(uuid.NewV7()).String(),
				organizationId,
				newDecisionId,
				ruleExecution.ResultScoreModifier,
				ruleExecution.Result,
				ruleExecution.ExecutionError,
				ruleExecution.Rule.Id,
				serializedRuleEvaluation,
				ruleExecution.Outcome,
			)
	}
	err = ExecBuilder(ctx, exec, builderForRules)
	return err
}

func (repo *MarbleDbRepository) UpdateDecisionCaseId(ctx context.Context, exec Executor, decisionIds []string, caseId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_DECISIONS).
		Set("case_id", caseId).
		Where(squirrel.Eq{"id": decisionIds})

	err := ExecBuilder(ctx, exec, query)
	return err
}

func selectJoinDecisionAndCase() squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("d", dbmodels.SelectDecisionColumn)...)
	columns = append(columns, columnsNames("c", dbmodels.SelectCaseColumn)...)
	return NewQueryBuilder().
		Select(columns...).
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		LeftJoin(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
		OrderBy("d.created_at DESC")
}

func (repo *MarbleDbRepository) rulesOfDecisions(
	ctx context.Context,
	exec Executor,
	decisionIds []string,
) (map[string][]models.RuleExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	rules, err := SqlToListOfRow(
		ctx,
		exec,
		NewQueryBuilder().
			Select("d.id, d.org_id, d.decision_id, r.name, r.description, d.score_modifier, d.result, d.error_code, d.rule_id, d.rule_evaluation, d.outcome").
			From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISION_RULES)).
			Join(fmt.Sprintf("%s AS r ON d.rule_id = r.id", dbmodels.TABLE_RULES)).
			Where(squirrel.Eq{"decision_id": decisionIds}).
			OrderBy("d.id"),
		func(row pgx.CollectableRow) (models.RuleExecution, error) {
			var r dbmodels.DbDecisionRule
			err := row.Scan(
				&r.Id,
				&r.OrganizationId,
				&r.DecisionId,
				&r.Name,
				&r.Description,
				&r.ScoreModifier,
				&r.Result,
				&r.ErrorCode,
				&r.RuleId,
				&r.RuleEvaluation,
				&r.Outcome,
			)
			if err != nil {
				return models.RuleExecution{}, err
			}

			return dbmodels.AdaptRuleExecution(r)
		},
	)
	if err != nil {
		return nil, err
	}

	rulesAsMap := make(map[string][]models.RuleExecution, len(rules))
	for _, rule := range rules {
		rulesAsMap[rule.DecisionId] = append(rulesAsMap[rule.DecisionId], rule)
	}
	return rulesAsMap, err
}

type RulesOfDecision struct {
	rules []models.RuleExecution
}

// Return an array of RulesOfDecision that correspond to the decisionIds
func (repo *MarbleDbRepository) rulesOfDecisionsBatch(ctx context.Context, exec Executor, decisionIds []string) ([]RulesOfDecision, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	allRules, err := SqlToListOfRow(
		ctx,
		exec,
		NewQueryBuilder().
			Select("d.id, d.org_id, d.decision_id, r.name, r.description, d.score_modifier, d.result, d.error_code, d.rule_id, d.rule_evaluation, d.outcome").
			From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISION_RULES)).
			Join(fmt.Sprintf("%s AS r ON d.rule_id = r.id", dbmodels.TABLE_RULES)).
			Where(squirrel.Eq{"decision_id": decisionIds}).
			OrderBy("decision_id"),
		func(row pgx.CollectableRow) (models.RuleExecution, error) {
			var r dbmodels.DbDecisionRule
			err := row.Scan(
				&r.Id,
				&r.OrganizationId,
				&r.DecisionId,
				&r.Name,
				&r.Description,
				&r.ScoreModifier,
				&r.Result,
				&r.ErrorCode,
				&r.RuleId,
				&r.RuleEvaluation,
				&r.Outcome,
			)
			if err != nil {
				return models.RuleExecution{}, err
			}

			return dbmodels.AdaptRuleExecution(r)
		},
	)
	if err != nil {
		return nil, err
	}

	decisionsRulesMap := make(map[string]*RulesOfDecision, len(decisionIds))
	for _, decisionId := range decisionIds {
		decisionsRulesMap[decisionId] = &RulesOfDecision{}
	}

	// dispatch rules to their corresponding decision
	for _, rule := range allRules {
		rulesOfDecision := decisionsRulesMap[rule.DecisionId]
		rulesOfDecision.rules = append(rulesOfDecision.rules, rule)
	}

	// return an array of RulesOfDecision that match the input array decisionIds
	return pure_utils.Map(decisionIds, func(decisionId string) RulesOfDecision {
		return *decisionsRulesMap[decisionId]
	}), nil
}

func (repo *MarbleDbRepository) channelOfDecisions(
	ctx context.Context,
	exec Executor,
	query squirrel.Sqlizer,
) (<-chan models.DecisionWithRuleExecutions, <-chan error) {
	decisionsChannel := make(chan models.DecisionWithRuleExecutions, 100)
	errChannel := make(chan error, 1)

	go func() {
		defer close(decisionsChannel)
		defer close(errChannel)

		dbDecisionsChannel, dbErrChannel := SqlToChannelOfModels(
			ctx,
			exec,
			query,
			func(
				row pgx.CollectableRow,
			) (dbmodels.DbDecision, error) {
				return pgx.RowToStructByName[dbmodels.DbDecision](row)
			},
		)

		var allErrors []error

		for dbDecisions := range BatchChannel(dbDecisionsChannel, decisionRulesBatchSize) {

			// fetch rules of all decisions
			rules, err := repo.rulesOfDecisionsBatch(
				ctx,
				exec,
				pure_utils.Map(dbDecisions, func(d dbmodels.DbDecision) string { return d.Id }),
			)
			if err != nil {
				allErrors = append(allErrors, err)
				// do not send invalid decisions
				continue
			}

			for i := 0; i < len(dbDecisions); i++ {
				decisionsChannel <- dbmodels.AdaptDecisionWithRuleExecutions(dbDecisions[i], rules[i].rules, nil)
			}
		}

		// wait for Db to finish
		allErrors = append(allErrors, <-dbErrChannel)

		errChannel <- errors.Join(allErrors...)
	}()

	return decisionsChannel, errChannel
}

func selectDecisions() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectDecisionColumn...).
		From(dbmodels.TABLE_DECISIONS)
}

func BatchChannel[Value any](inChannel <-chan Value, batchSize int) <-chan []Value {
	out := make(chan []Value, batchSize)

	go func() {
		defer close(out)

		buf := make([]Value, 0, batchSize)

		flush := func() {
			if len(buf) > 0 {
				out <- buf
				buf = make([]Value, 0, batchSize)
			}
		}

		for v := range inChannel {
			buf = append(buf, v)
			if len(buf) == batchSize {
				flush()
			}
		}

		flush()
	}()

	return out
}

func ChanToSlice[Model any](channel <-chan Model) []Model {
	slice := make([]Model, 0)
	for item := range channel {
		slice = append(slice, item)
	}
	return slice
}

func (repo *MarbleDbRepository) ReviewDecision(ctx context.Context, exec Executor, decisionId string, reviewStatus string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_DECISIONS).
		Set("review_status", reviewStatus).
		Where(squirrel.Eq{"id": decisionId})

	err := ExecBuilder(ctx, exec, query)
	return err
}

// Returns a DISTINCT set of (pivot_id, pivot_value) pairs from decisions for the given caseId, with count
func (repo *MarbleDbRepository) DecisionPivotValuesByCase(ctx context.Context, exec Executor, caseId string) ([]models.PivotDataWithCount, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("pivot_id, pivot_value, count(*) as nb").
		From(dbmodels.TABLE_DECISIONS).
		Where(squirrel.Eq{"case_id": caseId}).
		Where(squirrel.NotEq{"pivot_id": nil, "pivot_value": nil}).
		GroupBy("pivot_id, pivot_value")

	out, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.PivotDataWithCount, error) {
		var pivotData models.PivotDataWithCount
		err := row.Scan(&pivotData.PivotId, &pivotData.PivotValue, &pivotData.NbOfDecisions)
		if err != nil {
			return models.PivotDataWithCount{}, err
		}
		return pivotData, nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading pivot values in DecisionPivotValuesByCase")
	}
	return out, nil
}
