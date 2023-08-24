package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Undefined struct {
}

func (f Undefined) Evaluate(arguments ast.Arguments) (any, []error) {
	return MakeEvaluateError(fmt.Errorf("function Undefined %w", ast.ErrUndefinedFunction))
}
