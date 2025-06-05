package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type ContainsAny struct {
	Function ast.Function
}

func NewContainsAny(f ast.Function) ContainsAny {
	return ContainsAny{
		Function: f,
	}
}

func (f ContainsAny) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}
	if leftAny == nil || rightAny == nil {
		return nil, nil
	}

	left, err := adaptArgumentToString(leftAny)
	if err != nil {
		return MakeEvaluateError(err)
	}

	right, err := adaptArgumentToListOfStrings(rightAny)
	if err != nil {
		return MakeEvaluateError(err)
	}

	var containsElement bool
	for _, r := range right {
		if strings.Contains(strings.ToLower(left), strings.ToLower(r)) {
			containsElement = true
			break
		}
	}

	switch f.Function {
	case ast.FUNC_CONTAINS_ANY:
		return containsElement, nil
	case ast.FUNC_CONTAINS_NONE:
		return !containsElement, nil
	default:
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"ContainsAny does not support %s function", f.Function.DebugString())))
	}
}
