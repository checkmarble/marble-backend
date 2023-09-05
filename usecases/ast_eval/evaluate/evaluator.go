package evaluate

import "github.com/checkmarble/marble-backend/models/ast"

type Evaluator interface {
	Evaluate(arguments ast.Arguments) (any, []error)
}
