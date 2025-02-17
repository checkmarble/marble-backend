package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Masterminds/squirrel"
)

func selectRules() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectRulesColumn...).
		From(dbmodels.TABLE_RULES)
}

func (repo *MarbleDbRepository) GetRuleById(ctx context.Context, exec Executor, ruleId string) (models.Rule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Rule{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectRules().Where(squirrel.Eq{"id": ruleId}),
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) ListRulesByIterationId(ctx context.Context, exec Executor, iterationId string) ([]models.Rule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectRules().
			Where(squirrel.Eq{"scenario_iteration_id": iterationId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptRule,
	)
}

// This method expects to be run in a transaction, because we set some local settings
// that should not be changed for the whole connection.
func (repo *MarbleDbRepository) RulesExecutionStats(
	ctx context.Context,
	exec Transaction,
	organizationId string,
	iterationId string,
	begin, end time.Time,
) ([]models.RuleExecutionStat, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// The following settings are set because in some cases at least, the query planner
	// has ended up choosing a query plan involving a hash join with a full table scan on decision_rules
	_, err := exec.Exec(ctx,
		`SET local join_collapse_limit = 1;
		SET local enable_hashjoin = off;
		SET local enable_mergejoin = off;`)
	if err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("scir.stable_rule_id, scir.name, dr.outcome, scit.version, COUNT(*) as total").
		From("decisions as d").
		Join("scenario_iterations as scit ON scit.id = d.scenario_iteration_id").
		Join("scenario_iteration_rules as scir ON scir.scenario_iteration_id = scit.id").
		Join("decision_rules as dr ON dr.rule_id = scir.id and dr.decision_id = d.id").
		Where(squirrel.GtOrEq{"d.created_at": begin}).
		Where(squirrel.LtOrEq{"d.created_at": end}).
		Where(squirrel.Eq{
			"d.org_id":                organizationId,
			"d.scenario_iteration_id": iterationId,
		}).
		GroupBy("scir.stable_rule_id, scir.name, dr.outcome, scit.version")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptRuleExecutionStat,
	)
}

// This method expects to be run in a transaction, because we set some local settings
// that should not be changed for the whole connection.
func (repo *MarbleDbRepository) PhanomRulesExecutionStats(
	ctx context.Context,
	exec Transaction,
	organizationId string,
	iterationId string,
	begin, end time.Time,
) ([]models.RuleExecutionStat, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// The following settings are set because in some cases at least, the query planner
	// has ended up choosing a query plan involving a hash join with a full table scan on decision_rules
	_, err := exec.Exec(ctx,
		`SET local join_collapse_limit = 1;
		SET local enable_hashjoin = off;
		SET local enable_mergejoin = off;`)
	if err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("scir.stable_rule_id, scir.name, dr.outcome, scit.version, COUNT(*) as total").
		From("phantom_decisions as d").
		Join("scenario_iterations as scit ON scit.id = d.scenario_iteration_id").
		Join("scenario_iteration_rules as scir ON scir.scenario_iteration_id = scit.id").
		Join("decision_rules as dr ON dr.rule_id = scir.id and dr.decision_id = d.id").
		Where(squirrel.GtOrEq{"d.created_at": begin}).
		Where(squirrel.LtOrEq{"d.created_at": end}).
		Where(squirrel.Eq{
			"d.org_id":                organizationId,
			"d.scenario_iteration_id": iterationId,
		}).
		GroupBy("scir.stable_rule_id, scir.name, dr.outcome, scit.version")

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptRuleExecutionStat,
	)
}

func (repo *MarbleDbRepository) SanctionCheckExecutionStats(
	ctx context.Context,
	exec Executor,
	organizationId string,
	iterationId string,
	begin, end time.Time,
	fakeStableRuleId string,
	base string, // "decisions" or "phantom_decisions"
) ([]models.RuleExecutionStat, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	var baseTable string
	switch base {
	case "decisions":
		baseTable = "decisions"
	case "phantom_decisions":
		baseTable = "phantom_decisions"
	default:
		return nil, errors.Newf("invalid base table in SanctionCheckExecutionStats: %s", base)
	}

	sqlVersion := "SELECT version FROM scenario_iterations WHERE id = $1"
	var version string
	err := exec.QueryRow(ctx, sqlVersion, iterationId).Scan(&version)
	if err != nil {
		return nil, err
	}

	sanctionCheckRuleName := "SELECT name FROM sanction_check_configs WHERE scenario_iteration_id = $1"
	var name string
	err = exec.QueryRow(ctx, sanctionCheckRuleName, iterationId).Scan(&name)
	// All iterations don't have a sanction check config enabled. If there is none, just early exit
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// gathers statistics based on the initial sanction check match for each decision (is_manual = false) and
	// based on the initial status (has matches to review or none)
	query := NewQueryBuilder().
		Select("CASE WHEN sc.initial_has_matches THEN 'hit' ELSE 'no_hit' END AS outcome, COUNT(*) as total").
		From(fmt.Sprintf("%s as d", baseTable)).
		Join("sanction_checks as sc ON sc.decision_id = d.id").
		Where(squirrel.GtOrEq{"d.created_at": begin}).
		Where(squirrel.LtOrEq{"d.created_at": end}).
		Where(squirrel.Eq{"d.org_id": organizationId}).
		Where("sc.is_manual = false").
		GroupBy("1")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.RuleExecutionStat
	for rows.Next() {
		var stat models.RuleExecutionStat
		err = rows.Scan(&stat.Outcome, &stat.Total)
		if err != nil {
			return nil, err
		}
		stat.Name = name
		stat.Version = version
		stat.StableRuleId = &fakeStableRuleId
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

func (repo *MarbleDbRepository) UpdateRule(ctx context.Context, exec Executor, rule models.UpdateRuleInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	dbUpdateRuleInput, err := dbmodels.AdaptDBUpdateRuleInput(rule)
	if err != nil {
		return err
	}

	updateRequest := NewQueryBuilder().
		Update(dbmodels.TABLE_RULES).
		SetMap(utils.ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id)

	err = ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *MarbleDbRepository) DeleteRule(ctx context.Context, exec Executor, ruleID string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}

func (repo *MarbleDbRepository) CreateRules(ctx context.Context, exec Executor, rules []models.CreateRuleInput) ([]models.Rule, error) {
	if len(rules) == 0 {
		return []models.Rule{}, fmt.Errorf("no rule found")
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].StableRuleId == nil {
			newId := uuid.NewString()
			rules[i].StableRuleId = &newId
		}
	}

	dbCreateRuleInputs, err := pure_utils.MapErr(rules, dbmodels.AdaptDBCreateRuleInput)
	if err != nil {
		return []models.Rule{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_RULES).
		Columns(
			"id",
			"scenario_iteration_id",
			"org_id",
			"display_order",
			"name",
			"description",
			"formula_ast_expression",
			"score_modifier",
			"rule_group",
			"snooze_group_id",
			"stable_rule_id",
		).
		Suffix("RETURNING *")

	for _, rule := range dbCreateRuleInputs {
		query = query.Values(
			rule.Id,
			rule.ScenarioIterationId,
			rule.OrganizationId,
			rule.DisplayOrder,
			rule.Name,
			rule.Description,
			rule.FormulaAstExpression,
			rule.ScoreModifier,
			rule.RuleGroup,
			rule.SnoozeGroupId,
			rule.StableRuleId,
		)
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) CreateRule(ctx context.Context, exec Executor, rule models.CreateRuleInput) (models.Rule, error) {
	rules, err := repo.CreateRules(ctx, exec, []models.CreateRuleInput{rule})
	if err != nil {
		return models.Rule{}, err
	}
	return rules[0], nil
}
