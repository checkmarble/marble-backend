package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type List struct{}

func (l List) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	return arguments.Args, nil
}
