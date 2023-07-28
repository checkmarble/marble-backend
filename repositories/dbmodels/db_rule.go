package dbmodels

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type DBRule struct {
	ID                   string      `db:"id"`
	OrgID                string      `db:"org_id"`
	ScenarioIterationID  string      `db:"scenario_iteration_id"`
	DisplayOrder         int         `db:"display_order"`
	Name                 string      `db:"name"`
	Description          string      `db:"description"`
	ScoreModifier        int         `db:"score_modifier"`
	FormulaAstExpression []byte      `db:"formula_ast_expression"`
	CreatedAt            time.Time   `db:"created_at"`
	DeletedAt            pgtype.Time `db:"deleted_at"`
}

func AdaptRule(db DBRule) (models.Rule, error) {

	formulaAstExpression, err := AdaptSerializedAstExpression(db.FormulaAstExpression)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to unmarshal formula ast expression: %w", err)
	}

	return models.Rule{
		ID:                   db.ID,
		ScenarioIterationID:  db.ScenarioIterationID,
		OrganizationId:       db.OrgID,
		DisplayOrder:         db.DisplayOrder,
		Name:                 db.Name,
		Description:          db.Description,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        db.ScoreModifier,
		CreatedAt:            db.CreatedAt,
	}, nil
}

const TABLE_RULES = "scenario_iteration_rules"

var SelectRulesColumn = utils.ColumnList[DBRule]()
