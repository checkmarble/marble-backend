package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type DbDecisionRule struct {
	Id             string             `db:"id"`
	OrganizationId string             `db:"org_id"` //nolint:tagliatelle
	DecisionId     string             `db:"decision_id"`
	Name           string             `db:"-"`
	Description    string             `db:"-"`
	ScoreModifier  int                `db:"score_modifier"`
	Result         bool               `db:"result"`
	ErrorCode      ast.ExecutionError `db:"error_code"`
	RuleId         string             `db:"rule_id"`
	RuleEvaluation []byte             `db:"rule_evaluation"`
	Outcome        string             `db:"outcome"`
}

const TABLE_DECISION_RULES = "decision_rules"

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
		DecisionId:          db.DecisionId,
		ExecutionError:      db.ErrorCode,
		Evaluation:          evaluation,
		Outcome:             outcome,
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Rule: models.Rule{
			Id:          db.RuleId,
			Name:        db.Name,
			Description: db.Description,
		},
	}, nil
}

type DbOffloadableDecisionRule struct {
	// Decision
	DecisionId string    `db:"id"`
	CreatedAt  time.Time `db:"created_at"`

	// Rule execution
	RuleExecutionId *string `db:"rule_execution_id"`
	RuleId          *string `db:"rule_id"`
	RuleEvaluation  []byte  `db:"rule_evaluation"`
}

func AdaptOffloadableRuleExecution(db DbOffloadableDecisionRule) (models.OffloadableDecisionRule, error) {
	evaluation, err := DeserializeNodeEvaluationDto(db.RuleEvaluation)
	if err != nil {
		return models.OffloadableDecisionRule{}, err
	}

	return models.OffloadableDecisionRule{
		DecisionId:      db.DecisionId,
		CreatedAt:       db.CreatedAt,
		RuleExecutionId: db.RuleExecutionId,
		RuleId:          db.RuleId,
		RuleEvaluation:  evaluation,
	}, nil
}
