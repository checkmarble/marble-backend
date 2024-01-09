package evaluate

import (
	"context"
	"fmt"
	"strings"

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

	left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToString)

	if len(errs) > 0 {
		return nil, errs
	}

	if f.Function == ast.FUNC_STRING_CONTAINS {
		return strings.Contains(strings.ToLower(left), strings.ToLower(right)), nil
	} else if f.Function == ast.FUNC_STRING_NOT_CONTAIN {
		return !strings.Contains(strings.ToLower(left), strings.ToLower(right)), nil
	} else {
		return MakeEvaluateError(fmt.Errorf("StringContains does not support %s function", f.Function.DebugString()))
	}
}
