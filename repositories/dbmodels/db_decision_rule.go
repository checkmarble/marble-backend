package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DbDecisionRule struct {
	Id             string             `db:"id"`
	OrganizationId string             `db:"org_id"`
	DecisionId     string             `db:"decision_id"`
	Name           string             `db:"name"`
	Description    string             `db:"description"`
	ScoreModifier  int                `db:"score_modifier"`
	Result         bool               `db:"result"`
	ErrorCode      ast.ExecutionError `db:"error_code"`
	DeletedAt      pgtype.Time        `db:"deleted_at"`
	RuleId         string             `db:"rule_id"`
	RuleEvaluation []byte             `db:"rule_evaluation"`
	Outcome        string             `db:"outcome"`
}

const TABLE_DECISION_RULES = "decision_rules"

var SelectDecisionRuleColumn = utils.ColumnList[DbDecisionRule]()

func AdaptRuleExecution(db DbDecisionRule) (models.RuleExecution, error) {
	evaluation, err := DeserializeNodeEvaluationDto(db.RuleEvaluation)
	if err != nil {
		return models.RuleExecution{}, err
	}

	outcome := db.Outcome
	if outcome == "" {
		if db.ErrorCode != 0 {
			outcome = "error"
		} else if db.Result {
			outcome = "hit"
		} else {
			outcome = "no_hit"
		}
	}
	return models.RuleExecution{
		DecisionId: db.DecisionId,
		Rule: models.Rule{
			Id:          db.RuleId,
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               ast.AdaptErrorCodeAsError(db.ErrorCode),
		Evaluation:          evaluation,
		Outcome:             outcome,
	}, nil
}
