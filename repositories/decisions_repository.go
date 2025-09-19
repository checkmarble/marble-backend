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
	DecisionsByCaseIdFromCursor(
		ctx context.Context,
		exec Executor,
		req models.CaseDecisionsRequest,
	) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error)
	DecisionsByCaseId(
		ctx context.Context,
		exec Executor,
		orgId string,
		caseId string,
	) ([]models.Decision, error)

	// DEPRECATED: Do not use, see warning comment below next to the implementation
	DEPRECATED_DecisionsByObjectId(ctx context.Context, exec Executor, organizationId string,
		objectId string) ([]models.DecisionMetadata, error)

	StoreDecision(
		ctx context.Context,
		exec Executor,
		decision models.DecisionWithRuleExecutions,
		organizationId string,
		newDecisionId string) error
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

// To read a decision complete with basic case information and scenario details requires several joins, which is why we have this helper.
// It must be used with dbmodels.DbCoreDecisionWithCaseAndScenario.
// Very targeted repo methods that only need the core decision data may benefit from using a simpler query and model to scan to.
func selectDecisionAndCase() squirrel.SelectBuilder {
	columns := columnsNames("d", dbmodels.SelectCoreDecisionColumn)
	columns = append(columns, columnsNames("c", dbmodels.SelectCaseColumn)...)
	return NewQueryBuilder().
		Select(columns...).
		Column("s.name AS scenario_name").
		Column("s.description AS scenario_description").
		Column("si.version AS scenario_version").
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		LeftJoin(fmt.Sprintf("%s AS c ON c.id = d.case_id", dbmodels.TABLE_CASES)).
		LeftJoin(fmt.Sprintf("%s AS s on d.scenario_id = s.id", dbmodels.TABLE_SCENARIOS)).
		LeftJoin(fmt.Sprintf("%s AS si on d.scenario_iteration_id = si.id", dbmodels.TABLE_SCENARIO_ITERATIONS))
}

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
		si, err := repo.GetScenarioIteration(ctx, exec, scenarioLiveIterationId, true)
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

	rules, err := repo.rulesOfDecisions(ctx, exec, decisionIds, true)
	if err != nil {
		return nil, err
	}

	return SqlToListOfRow(
		ctx,
		exec,
		selectDecisionAndCase().
			Where(squirrel.Eq{"d.id": decisionIds}).
			OrderBy("d.created_at DESC, d.id DESC"),
		func(row pgx.CollectableRow) (models.DecisionWithRuleExecutions, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecisionWithCaseAndScenario](row)
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}

			decision, err := dbmodels.AdaptDecisionWithCase(db)
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}

			return models.DecisionWithRuleExecutions{
				Decision:       decision,
				RuleExecutions: rules[db.DbCoreDecision.Id.String()],
			}, nil
		},
	)
}

func (repo *MarbleDbRepository) DecisionsById(ctx context.Context, exec Executor, decisionIds []string) ([]models.Decision, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectDecisionAndCase().
		Where(squirrel.Eq{"d.id": decisionIds}).
		OrderBy("d.created_at DESC")

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecisionWithCaseAndScenario](row)
		if err != nil {
			return models.Decision{}, err
		}

		return dbmodels.AdaptDecisionWithCase(db)
	})
}

func (repo *MarbleDbRepository) DecisionsByCaseIdFromCursor(
	ctx context.Context,
	exec Executor,
	req models.CaseDecisionsRequest,
) (decisionsWithRules []models.DecisionWithRulesAndScreeningsBaseInfo, hasMore bool, err error) {
	if err = validateMarbleDbExecutor(exec); err != nil {
		return
	}

	query := NewQueryBuilder().
		Select(columnsNames("d", dbmodels.SelectCoreDecisionColumn)...).
		Column("s.name AS scenario_name").
		Column("s.description AS scenario_description").
		Column("si.version AS scenario_version").
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		LeftJoin(fmt.Sprintf("%s AS s on d.scenario_id = s.id", dbmodels.TABLE_SCENARIOS)).
		LeftJoin(fmt.Sprintf("%s AS si on d.scenario_iteration_id = si.id", dbmodels.TABLE_SCENARIO_ITERATIONS)).
		Where(squirrel.Eq{
			"d.org_id":  req.OrgId,
			"d.case_id": req.CaseId,
		}).
		OrderBy("d.created_at DESC, d.id DESC")
	if req.Limit > 0 {
		query = query.Limit(uint64(req.Limit) + 1)
	}

	if req.CursorId != "" {
		cursorDecision, err := repo.DecisionsById(ctx, exec, []string{req.CursorId})
		if err != nil {
			return nil, false, err
		}
		if len(cursorDecision) == 0 {
			return nil, false, errors.Wrap(models.NotFoundError, "could not find decision for cursor")
		}

		query = query.Where("(d.created_at, d.id) < (?, ?)", cursorDecision[0].CreatedAt, req.CursorId)
	}

	decisions, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecisionWithScenario](row)
		if err != nil {
			return models.Decision{}, err
		}
		return dbmodels.AdaptDecision(db)
	})
	if err != nil {
		return nil, false, err
	}

	var rules map[string][]models.RuleExecution
	var screenings map[string][]models.Screening

	decisionIds := pure_utils.Map(decisions, func(d models.Decision) string { return d.DecisionId.String() })
	rules, err = repo.rulesOfDecisions(ctx, exec, decisionIds, false)
	if err != nil {
		return nil, false, err
	}

	screenings, err = repo.screeningsWithoutHitsOfDecision(ctx, exec, decisionIds)
	if err != nil {
		return nil, false, err
	}

	decisionsWithRules = make([]models.DecisionWithRulesAndScreeningsBaseInfo, len(decisions))
	for i, decision := range decisions {
		decisionsWithRules[i] = models.DecisionWithRulesAndScreeningsBaseInfo{
			Decision:            decision,
			RuleExecutions:      rules[decision.DecisionId.String()],
			ScreeningExecutions: screenings[decision.DecisionId.String()],
		}
	}

	if req.Limit > 0 {
		hasMore = len(decisionsWithRules) > req.Limit
		decisionsWithRules = decisionsWithRules[:min(len(decisionsWithRules), req.Limit)]
		return
	}
	return decisionsWithRules, false, nil
}

func (repo *MarbleDbRepository) DecisionsByCaseId(
	ctx context.Context,
	exec Executor,
	orgId string,
	caseId string,
) ([]models.Decision, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(columnsNames("d", dbmodels.SelectCoreDecisionColumn)...).
		Column("s.name AS scenario_name").
		Column("s.description AS scenario_description").
		Column("si.version AS scenario_version").
		From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISIONS)).
		LeftJoin(fmt.Sprintf("%s AS s on d.scenario_id = s.id", dbmodels.TABLE_SCENARIOS)).
		LeftJoin(fmt.Sprintf("%s AS si on d.scenario_iteration_id = si.id", dbmodels.TABLE_SCENARIO_ITERATIONS)).
		Where(squirrel.Eq{
			"d.org_id":  orgId,
			"d.case_id": caseId,
		}).
		OrderBy("d.created_at DESC, d.id DESC")

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecisionWithScenario](row)
		if err != nil {
			return models.Decision{}, err
		}
		return dbmodels.AdaptDecision(db)
	})
}

// DEPRECATED: Do not use, see warning comment below
func (repo *MarbleDbRepository) DEPRECATED_DecisionsByObjectId(
	ctx context.Context,
	exec Executor,
	organizationId string,
	objectId string,
) ([]models.DecisionMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// WARNING: This query is broken with the change in which this comment is introduced.. Anyway, it's deprecated and only used in
	// the transfercheck usecase.
	// To be removed but in another follow-up PR. I am only not doing it in the same PR because there is a LOT of code to remove
	// which would be a distraction.
	query := selectDecisionAndCase().
		Where(squirrel.Eq{"d.org_id": organizationId}).
		Where(squirrel.Eq{"d.trigger_object->>'object_id'": objectId}).
		OrderBy("d.created_at DESC")

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.DecisionMetadata, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecision](row)
		if err != nil {
			return models.DecisionMetadata{}, err
		}

		return dbmodels.AdaptDecisionMetadata(db), nil
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

	orderCond := fmt.Sprintf("d.%s %s, d.id %s", pagination.Sorting, pagination.Order, pagination.Order)

	query := selectDecisionAndCase().
		Where(squirrel.Eq{"d.org_id": organizationId}).
		OrderBy(orderCond).
		Limit(uint64(pagination.Limit))

	// Add filters
	if len(filters.ScenarioIds) > 0 {
		query = query.Where(squirrel.Eq{"d.scenario_id": filters.ScenarioIds})
	}
	if len(filters.Outcomes) > 0 {
		query = query.Where(squirrel.Eq{"d.outcome": filters.Outcomes})
	}
	if len(filters.TriggerObjects) > 0 {
		query = query.Where(squirrel.Eq{"d.trigger_object_type": filters.TriggerObjects})
	}
	if filters.TriggerObjectId != nil {
		query = query.Where(squirrel.Eq{"d.trigger_object->>'object_id'": *filters.TriggerObjectId})
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
	// This filter condition relies on the join with the cases table
	if len(filters.CaseInboxIds) > 0 {
		query = query.Where(squirrel.Eq{"c.inbox_id": filters.CaseInboxIds})
	}

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
	var err error
	query, err = applyDecisionPaginationFilters(query, pagination, offsetDecision.Decision)
	if err != nil {
		return nil, err
	}

	decision, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Decision, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DbCoreDecisionWithCaseAndScenario](row)
		if err != nil {
			return models.Decision{}, err
		}

		return dbmodels.AdaptDecisionWithCase(db)
	})
	if err != nil {
		return nil, err
	}
	return decision, nil
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

	args := []any{offsetValue, p.OffsetId}
	if p.Order == models.SortingOrderDesc {
		query = query.Where(fmt.Sprintf("(d.%s, d.id) < (?, ?)", p.Sorting), args...)
	} else {
		query = query.Where(fmt.Sprintf("(d.%s, d.id) > (?, ?)", p.Sorting), args...)
	}

	return query, nil
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

func (repo *MarbleDbRepository) rulesOfDecisions(
	ctx context.Context,
	exec Executor,
	decisionIds []string,
	withEvaluation bool,
) (map[string][]models.RuleExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	columns := "d.id, d.org_id, d.decision_id, r.name, r.description, d.score_modifier, d.result, d.error_code, d.rule_id, d.outcome"
	if withEvaluation {
		columns += ", d.rule_evaluation"
	}
	rules, err := SqlToListOfRow(
		ctx,
		exec,
		NewQueryBuilder().
			Select(columns).
			From(fmt.Sprintf("%s AS d", dbmodels.TABLE_DECISION_RULES)).
			Join(fmt.Sprintf("%s AS r ON d.rule_id = r.id", dbmodels.TABLE_RULES)).
			Where(squirrel.Eq{"decision_id": decisionIds}).
			OrderBy("d.id"),
		func(row pgx.CollectableRow) (models.RuleExecution, error) {
			var r dbmodels.DbDecisionRule
			fields := []any{
				&r.Id,
				&r.OrganizationId,
				&r.DecisionId,
				&r.Name,
				&r.Description,
				&r.ScoreModifier,
				&r.Result,
				&r.ErrorCode,
				&r.RuleId,
				&r.Outcome,
			}
			if withEvaluation {
				fields = append(fields, &r.RuleEvaluation)
			}
			err := row.Scan(fields...)
			if err != nil {
				return models.RuleExecution{}, err
			}

			return dbmodels.AdaptRuleExecution(r)
		},
	)
	if err != nil {
		return nil, err
	}

	rulesAsMap := make(map[string][]models.RuleExecution, len(decisionIds))
	for _, rule := range rules {
		rulesAsMap[rule.DecisionId] = append(rulesAsMap[rule.DecisionId], rule)
	}
	return rulesAsMap, err
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

func (repo *MarbleDbRepository) GetOffloadableDecisionRules(
	ctx context.Context,
	tx Transaction,
	req models.OffloadDecisionRuleRequest,
) (models.ChannelOfModels[models.OffloadableDecisionRule], error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.ChannelOfModels[models.OffloadableDecisionRule]{}, err
	}

	if req.Watermark == nil {
		req.Watermark = &models.Watermark{
			WatermarkTime: time.Time{},
			WatermarkId:   utils.Ptr(uuid.UUID{}.String()),
		}
	}

	if req.Watermark.WatermarkId == nil {
		return models.ChannelOfModels[models.OffloadableDecisionRule]{}, errors.New("watermark id is required")
	}

	// In this query, the query planner may choose a hash join, which is about guaranteed to never complete on this large table.
	_, err := tx.Exec(ctx, "SET local enable_hashjoin = off;")
	if err != nil {
		return models.ChannelOfModels[models.OffloadableDecisionRule]{}, err
	}

	inequalitySymbol := ">"
	if req.LargeInequality {
		inequalitySymbol = ">="
	}

	sql := NewQueryBuilder().
		Select(
			"d.id as id",
			"d.created_at as created_at",
			"dr.id as rule_execution_id",
			"dr.rule_id as rule_id",
			"dr.outcome as outcome",
			"dr.rule_evaluation as rule_evaluation",
		).
		From(dbmodels.TABLE_DECISIONS + " AS d").
		Join(fmt.Sprintf("%s AS dr ON dr.decision_id = d.id", dbmodels.TABLE_DECISION_RULES)).
		Where(squirrel.And{
			squirrel.Eq{"d.org_id": req.OrgId},
			squirrel.Lt{"d.created_at": req.DeleteBefore},
			squirrel.Expr(fmt.Sprintf("(d.created_at, d.id) %s (?, ?)", inequalitySymbol),
				req.Watermark.WatermarkTime, *req.Watermark.WatermarkId),
		}).
		OrderBy("d.created_at, d.id").
		Limit(uint64(req.BatchSize))

	cb := func(row pgx.CollectableRow) (models.OffloadableDecisionRule, error) {
		dbRow, err := pgx.RowToStructByName[dbmodels.DbOffloadableDecisionRule](row)
		if err != nil {
			return models.OffloadableDecisionRule{}, err
		}
		return dbmodels.AdaptOffloadableRuleExecution(dbRow)
	}

	return SqlToChannelOfModel(ctx, tx, sql, cb), nil
}

func (repo *MarbleDbRepository) RemoveDecisionRulePayload(ctx context.Context, tx Transaction, ids []*string) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_DECISION_RULES).
		Set("rule_evaluation", nil).
		Set("outcome", squirrel.Expr("case outcome when '' then 'no_hit' else outcome end")).
		Where(squirrel.Eq{"id": ids})

	return ExecBuilder(ctx, tx, sql)
}

// Counts the number of decisions for each orgId in the given time range
func (repo *MarbleDbRepository) CountDecisionsByOrg(ctx context.Context, exec Executor,
	orgIds []string, from, to time.Time,
) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return map[string]int{}, err
	}

	query := NewQueryBuilder().
		Select("org_id, count(*) as count").
		From(dbmodels.TABLE_DECISIONS).
		Where(squirrel.Eq{"org_id": orgIds}).
		Where(squirrel.GtOrEq{"created_at": from}).
		Where(squirrel.Lt{"created_at": to}).
		GroupBy("org_id")

	return countByHelper(ctx, exec, query, orgIds)
}
