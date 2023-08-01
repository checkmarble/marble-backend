package scenarios

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval"
	"time"
)

func DryRunValue(table models.Field) any {
	switch table.DataType {
	case models.Bool:
		return true
	case models.String:
		return "dummy"
	case models.Int:
		return 1
	case models.Float:
		return 1.0
	case models.Timestamp:
		t, _ := time.Parse(time.RFC3339, time.RFC3339)
		return t
	default:
		return nil
	}
}

func DryRunPayload(table models.Table) map[string]any {

	result := make(map[string]any)
	for fieldName, field := range table.Fields {
		result[string(fieldName)] = DryRunValue(field)
	}

	return result
}

func DryRunAst(environment ast_eval.AstEvaluationEnvironment, node ast.Node) ast.NodeEvaluation {

	evaluation := ast_eval.EvaluateAst(environment, node)
	return evaluation
}
