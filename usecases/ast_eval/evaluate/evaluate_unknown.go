package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Unknown struct {
}

func (f Unknown) Evaluate(arguments ast.Arguments) (any, []error) {
	return MakeEvaluateError(fmt.Errorf("function Unknown %w", ast.ErrUnknownFunction))
}
