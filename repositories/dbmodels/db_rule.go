package dbmodels

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/models/operators"
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
	Formula              []byte      `db:"formula"`
	FormulaAstExpression []byte      `db:"formula_ast_expression"`
	CreatedAt            time.Time   `db:"created_at"`
	DeletedAt            pgtype.Time `db:"deleted_at"`
}

func AdaptRule(db DBRule) (models.Rule, error) {

	var formula *operators.OperatorBool
	if string(db.Formula) != "{}" {
		f, err := operators.UnmarshalOperatorBool(db.Formula)
		if err != nil {
			return models.Rule{}, fmt.Errorf("unable to unmarshal rule: %w", err)
		}
		formula = &f
	}

	var formulaAstExpression *ast.Node

	if len(db.FormulaAstExpression) != 0 {
		var serialized dto.NodeDto
		if err := json.Unmarshal(db.FormulaAstExpression, &serialized); err != nil {
			return models.Rule{}, err
		}

		node, err := dto.AdaptASTNode(serialized)
		if err != nil {
			return models.Rule{}, err
		}
		formulaAstExpression = &node
	}

	return models.Rule{
		ID:                   db.ID,
		ScenarioIterationID:  db.ScenarioIterationID,
		OrganizationId:       db.OrgID,
		DisplayOrder:         db.DisplayOrder,
		Name:                 db.Name,
		Description:          db.Description,
		Formula:              formula,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        db.ScoreModifier,
		CreatedAt:            db.CreatedAt,
	}, nil
}

const TABLE_RULES = "scenario_iteration_rules"

var SelectScenarioIterationRulesColumn = utils.ColumnList[DBRule]()
