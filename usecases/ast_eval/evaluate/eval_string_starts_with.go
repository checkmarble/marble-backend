package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringStartsWith struct {
	Function ast.Function
}

func NewStringStartsWith(f ast.Function) StringStartsWith {
	return StringStartsWith{
		Function: f,
	}
}

func (f StringStartsWith) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
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

	if f.Function == ast.FUNC_STRING_STARTS_WITH {
		return strings.HasPrefix(strings.ToLower(left), strings.ToLower(right)), nil
	} else {
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"StringStartsWith does not support %s function", f.Function.DebugString())))
	}
}
