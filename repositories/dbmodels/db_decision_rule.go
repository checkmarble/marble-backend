package dbmodels

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DbDecisionRule struct {
	Id             string      `db:"id"`
	OrganizationId string      `db:"org_id"`
	DecisionId     string      `db:"decision_id"`
	Name           string      `db:"name"`
	Description    string      `db:"description"`
	ScoreModifier  int         `db:"score_modifier"`
	Result         bool        `db:"result"`
	ErrorCode      int         `db:"error_code"`
	DeletedAt      pgtype.Time `db:"deleted_at"`
	RuleId         string      `db:"rule_id"`
	RuleEvaluation []byte      `db:"rule_evaluation"`
}

const TABLE_DECISION_RULES = "decision_rules"

var SelectDecisionRuleColumn = utils.ColumnList[DbDecisionRule]()

func adaptErrorCodeAsError(errCode models.ExecutionError) error {
	switch errCode {
	case models.NoError:
		return nil
	case models.NullFieldRead:
		return ast.ErrNullFieldRead
	case models.NoRowsRead:
		return ast.ErrNoRowsRead
	case models.DivisionByZero:
		return ast.ErrDivisionByZero
	case models.PayloadFieldNotFound:
		return ast.ErrPayloadFieldNotFound
	default:
		return fmt.Errorf("unknown error code")
	}
}

func AdaptRuleExecution(db DbDecisionRule) (models.RuleExecution, error) {
	evaluation, err := DeserializeNodeEvaluationDto(db.RuleEvaluation)
	if err != nil {
		return models.RuleExecution{}, err
	}

	return models.RuleExecution{
		Rule: models.Rule{
			Id:          db.RuleId,
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               adaptErrorCodeAsError(models.ExecutionError(db.ErrorCode)),
		Evaluation:          evaluation,
	}, nil
}
