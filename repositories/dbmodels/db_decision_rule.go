package dbmodels

import (
	"errors"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5/pgtype"
)

type DbDecisionRule struct {
	Id             string         `db:"id"`
	OrganizationId string         `db:"org_id"`
	DecisionId     string         `db:"decision_id"`
	Name           string         `db:"name"`
	Description    string         `db:"description"`
	ScoreModifier  int            `db:"score_modifier"`
	Result         bool           `db:"result"`
	ErrorCode      ExecutionError `db:"error_code"`
	DeletedAt      pgtype.Time    `db:"deleted_at"`
	RuleId         string         `db:"rule_id"`
	RuleEvaluation []byte         `db:"rule_evaluation"`
}

const TABLE_DECISION_RULES = "decision_rules"

var SelectDecisionRuleColumn = utils.ColumnList[DbDecisionRule]()

type ExecutionError int

const (
	NoError              ExecutionError = 0
	DivisionByZero       ExecutionError = 100
	NullFieldRead        ExecutionError = 200
	NoRowsRead           ExecutionError = 201
	PayloadFieldNotFound ExecutionError = 202
	Unknown              ExecutionError = -1
)

func (r ExecutionError) String() string {
	switch r {
	case DivisionByZero:
		return "A division by zero occurred in a rule"
	case NullFieldRead:
		return "A field read in a rule is null"
	case NoRowsRead:
		return "No rows were read from db in a rule"
	case PayloadFieldNotFound:
		return "A payload field was not found in a rule"
	case Unknown:
		return "Unknown error"
	}
	return ""
}

func AdaptExecutionError(err error) ExecutionError {
	switch {
	case err == nil:
		return NoError
	case errors.Is(err, ast.ErrNullFieldRead):
		return NullFieldRead
	case errors.Is(err, ast.ErrNoRowsRead):
		return NoRowsRead
	case errors.Is(err, ast.ErrDivisionByZero):
		return DivisionByZero
	case errors.Is(err, ast.ErrPayloadFieldNotFound):
		return PayloadFieldNotFound
	default:
		return Unknown
	}
}

func adaptErrorCodeAsError(errCode ExecutionError) error {
	switch errCode {
	case NoError:
		return nil
	case NullFieldRead:
		return ast.ErrNullFieldRead
	case NoRowsRead:
		return ast.ErrNoRowsRead
	case DivisionByZero:
		return ast.ErrDivisionByZero
	case PayloadFieldNotFound:
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
		Error:               adaptErrorCodeAsError(db.ErrorCode),
		Evaluation:          evaluation,
	}, nil
}
