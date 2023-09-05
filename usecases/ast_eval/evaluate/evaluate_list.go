package evaluate

import (
	"github.com/checkmarble/marble-backend/models/ast"
)

type List struct{}

func (l List) Evaluate(arguments ast.Arguments) (any, []error) {
	return arguments.Args, nil
}
