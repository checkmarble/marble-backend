package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Undefined struct {
}

func (f Undefined) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return MakeEvaluateError(fmt.Errorf("function Undefined %w", ast.ErrUndefinedFunction))
}
