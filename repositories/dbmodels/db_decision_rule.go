package dbmodels

import (
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

func AdaptRuleExecution(db DbDecisionRule) models.RuleExecution {
	return models.RuleExecution{
		Rule: models.Rule{
			Name:        db.Name,
			Description: db.Description,
		},
		Result:              db.Result,
		ResultScoreModifier: db.ScoreModifier,
		Error:               nil, // TODO put error handling by rule back in place
	}
}
