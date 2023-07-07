package evaluate

import (
	"marble/marble-backend/models/ast"
)

type Not struct {
	Function ast.Function
}

func (f Not) Evaluate(arguments ast.Arguments) (any, error) {

	if err := verifyNumberOfArguments(f.Function, arguments.Args, 1); err != nil {
		return nil, err
	}

	v, err := adaptArgumentToBool(f.Function, arguments.Args[0])
	if err != nil {
		return false, err
	}

	return !v, nil
}
