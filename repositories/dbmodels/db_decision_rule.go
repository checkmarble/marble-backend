package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type DbDecisionRule struct {
	Id             string
	OrganizationId string
	DecisionId     string
	Name           string
	Description    string
	ScoreModifier  int
	Result         bool
	ErrorCode      ast.ExecutionError
	RuleId         string
	RuleEvaluation []byte
	Outcome        string
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
		Error:               ast.AdaptErrorCodeAsError(db.ErrorCode),
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
