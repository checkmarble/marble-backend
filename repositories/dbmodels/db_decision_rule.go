package dbmodels

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
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
	RuleId         pgtype.Text `db:"rule_id"`
}

const TABLE_DECISION_RULE = "decision_rules"

var SelectDecisionRuleColumn = utils.ColumnList[DbDecisionRule]()

func adaptErrorCodeAsError(errCode models.RuleExecutionError) error {
	switch errCode {
	case models.NoError:
		return nil
	case models.NullFieldRead:
		return models.ErrNullFieldRead
	case models.NoRowsRead:
		return models.ErrNoRowsRead
	case models.DivisionByZero:
		return models.ErrDivisionByZero
	case models.PayloadFieldNotFound:
		return models.ErrPayloadFieldNotFound
	default:
		return fmt.Errorf("unknown error code")
	}
}

func AdaptRuleExecution(db DbDecisionRule) models.RuleExecution {
	return models.RuleExecution{
		Rule: models.Rule{
			Id:          db.RuleId.String,
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               adaptErrorCodeAsError(models.RuleExecutionError(db.ErrorCode)),
	}
}
