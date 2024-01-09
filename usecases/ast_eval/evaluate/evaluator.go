package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Evaluator interface {
	Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error)
}
