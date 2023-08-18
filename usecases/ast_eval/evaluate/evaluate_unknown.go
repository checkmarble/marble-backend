package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type Unknown struct {
}

func (f Unknown) Evaluate(arguments ast.Arguments) (any, error) {
	return nil, fmt.Errorf("function Unknown %w", models.ErrRuntimeExpression)
}
