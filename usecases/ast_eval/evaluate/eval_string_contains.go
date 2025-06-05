package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringContains struct {
	Function ast.Function
}

func NewStringContains(f ast.Function) StringContains {
	return StringContains{
		Function: f,
	}
}

func (f StringContains) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}
	if leftAny == nil || rightAny == nil {
		return nil, nil
	}

	left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToString)

	if len(errs) > 0 {
		return nil, errs
	}

	switch f.Function {
	case ast.FUNC_STRING_CONTAINS:
		return strings.Contains(strings.ToLower(left), strings.ToLower(right)), nil
	case ast.FUNC_STRING_NOT_CONTAIN:
		return !strings.Contains(strings.ToLower(left), strings.ToLower(right)), nil
	default:
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"StringContains does not support %s function", f.Function.DebugString())))
	}
}
