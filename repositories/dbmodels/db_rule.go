package dbmodels

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

const TABLE_RULES = "scenario_iteration_rules"

var SelectRulesColumn = utils.ColumnList[DBRule]()

type DBRule struct {
	Id                   string      `db:"id"`
	OrganizationId       string      `db:"org_id"`
	ScenarioIterationId  string      `db:"scenario_iteration_id"`
	DisplayOrder         int         `db:"display_order"`
	Name                 string      `db:"name"`
	Description          string      `db:"description"`
	ScoreModifier        int         `db:"score_modifier"`
	FormulaAstExpression []byte      `db:"formula_ast_expression"`
	CreatedAt            time.Time   `db:"created_at"`
	DeletedAt            pgtype.Time `db:"deleted_at"`
	RuleGroup            string      `db:"rule_group"`
	SnoozeGroupId        *string     `db:"snooze_group_id"`
	StableRuleId         *string     `db:"stable_rule_id"`
}

func AdaptRule(db DBRule) (models.Rule, error) {
	formulaAstExpression, err := AdaptSerializedAstExpression(db.FormulaAstExpression)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to unmarshal formula ast expression: %w", err)
	}

	return models.Rule{
		Id:                   db.Id,
		ScenarioIterationId:  db.ScenarioIterationId,
		OrganizationId:       db.OrganizationId,
		DisplayOrder:         db.DisplayOrder,
		Name:                 db.Name,
		Description:          db.Description,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        db.ScoreModifier,
		CreatedAt:            db.CreatedAt,
		RuleGroup:            db.RuleGroup,
		SnoozeGroupId:        db.SnoozeGroupId,
		StableRuleId:         db.StableRuleId,
	}, nil
}

type DbRuleExecution struct {
	StableRuleId *string `db:"stable_rule_id"`
	Name         string  `db:"name"`
	Version      string  `db:"version"`
	Outcome      string  `db:"outcome"`
	Total        int     `db:"total"`
}

func AdaptRuleExecutionStat(db DbRuleExecution) (models.RuleExecutionStat, error) {
	return models.RuleExecutionStat{
		Version:      db.Version,
		Name:         db.Name,
		Outcome:      db.Outcome,
		StableRuleId: *db.StableRuleId,
		Total:        db.Total,
	}, nil
}

type DBCreateRuleInput struct {
	Id                   string  `db:"id"`
	OrganizationId       string  `db:"org_id"`
	ScenarioIterationId  string  `db:"scenario_iteration_id"`
	DisplayOrder         int     `db:"display_order"`
	Name                 string  `db:"name"`
	Description          string  `db:"description"`
	ScoreModifier        int     `db:"score_modifier"`
	FormulaAstExpression *[]byte `db:"formula_ast_expression"`
	RuleGroup            string  `db:"rule_group"`
	SnoozeGroupId        *string `db:"snooze_group_id"`
	StableRuleId         *string `db:"stable_rule_id"`
}

func AdaptDBCreateRuleInput(rule models.CreateRuleInput) (DBCreateRuleInput, error) {
	formulaAstExpression, err := SerializeFormulaAstExpression(rule.FormulaAstExpression)
	if err != nil {
		return DBCreateRuleInput{}, fmt.Errorf("unable to marshal expression formula: %w", err)
	}

	return DBCreateRuleInput{
		Id:                   rule.Id,
		OrganizationId:       rule.OrganizationId,
		ScenarioIterationId:  rule.ScenarioIterationId,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		ScoreModifier:        rule.ScoreModifier,
		FormulaAstExpression: formulaAstExpression,
		RuleGroup:            rule.RuleGroup,
		SnoozeGroupId:        rule.SnoozeGroupId,
		StableRuleId:         rule.StableRuleId,
	}, nil
}

type DBUpdateRuleInput struct {
	Id                   string  `db:"id"`
	DisplayOrder         *int    `db:"display_order"`
	Name                 *string `db:"name"`
	Description          *string `db:"description"`
	ScoreModifier        *int    `db:"score_modifier"`
	FormulaAstExpression *[]byte `db:"formula_ast_expression"`
	RuleGroup            *string `db:"rule_group"`
	SnoozeGroupId        *string `db:"snooze_group_id"`
	StableRuleId         *string `db:"stable_rule_id"`
}

func AdaptDBUpdateRuleInput(rule models.UpdateRuleInput) (DBUpdateRuleInput, error) {
	formulaAstExpression, err := SerializeFormulaAstExpression(rule.FormulaAstExpression)
	if err != nil {
		return DBUpdateRuleInput{}, fmt.Errorf("unable to marshal expression formula: %w", err)
	}

	return DBUpdateRuleInput{
		Id:                   rule.Id,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		ScoreModifier:        rule.ScoreModifier,
		FormulaAstExpression: formulaAstExpression,
		RuleGroup:            rule.RuleGroup,
		SnoozeGroupId:        rule.SnoozeGroupId,
		StableRuleId:         rule.StableRuleId,
	}, nil
}
