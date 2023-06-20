package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Variable struct {
	Variables map[string]any
}

func (f Variable) Evaluate(arguments ast.Arguments) (any, error) {
	varname, err := arguments.StringNamedArgument(ast.AttributeFuncVariable.ArgumentVarname)
	if err != nil {
		return nil, err
	}

	if value, ok := f.Variables[varname]; ok {
		return value, nil
	}

	return 0, fmt.Errorf("variable does not exist: %s", varname)
}
