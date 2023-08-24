package evaluate

import "marble/marble-backend/models/ast"

type Evaluator interface {
	Evaluate(arguments ast.Arguments) (any, []error)
}
