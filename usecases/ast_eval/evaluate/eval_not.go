package evaluate

import (
	"github.com/checkmarble/marble-backend/models/ast"
)

type Not struct {
}

func (f Not) Evaluate(arguments ast.Arguments) (any, []error) {

	if err := verifyNumberOfArguments(arguments.Args, 1); err != nil {
		return MakeEvaluateError(err)
	}

	v, err := adaptArgumentToBool(arguments.Args[0])
	errs := MakeAdaptedArgsErrors([]error{err})
	if len(errs) > 0 {
		return nil, errs
	}

	return !v, nil
}
