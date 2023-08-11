package dbmodels

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

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
}

const TABLE_DECISION_RULE = "decision_rules"

var SelectDecisionRuleColumn = utils.ColumnList[DbDecisionRule]()

func adaptErrorCodeAsError(errCode models.RuleExecutionError) error {
	switch errCode {
	case models.NullFieldRead:
		return models.NullFieldReadError
	case models.NoRowsRead:
		return models.NoRowsReadError
	case models.DivisionByZero:
		return models.DivisionByZeroError
	default:
		return fmt.Errorf("unknown error code")
	}
}

func AdaptRuleExecution(db DbDecisionRule) models.RuleExecution {
	return models.RuleExecution{
		Rule: models.Rule{
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               adaptErrorCodeAsError(models.RuleExecutionError(db.ErrorCode)),
	}
}
