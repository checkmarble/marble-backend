package evaluate

import (
	"strings"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringContains struct{}

func (f StringContains) Evaluate(arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToString)

	if len(errs) > 0 {
		return nil, errs
	}

	return strings.Contains(strings.ToLower(left), strings.ToLower(right)), nil
}
