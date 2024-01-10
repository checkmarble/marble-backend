package evaluate

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Undefined struct {
}

func (f Undefined) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return MakeEvaluateError(errors.Wrap(ast.ErrUndefinedFunction, "Evaluate function Undefined"))
}
