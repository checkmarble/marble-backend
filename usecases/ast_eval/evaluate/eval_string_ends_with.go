package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringEndsWith struct {
	Function ast.Function
}

func NewStringEndsWith(f ast.Function) StringEndsWith {
	return StringEndsWith{
		Function: f,
	}
}

func (f StringEndsWith) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
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

	if f.Function == ast.FUNC_STRING_ENDS_WITH {
		return strings.HasSuffix(strings.ToLower(left), strings.ToLower(right)), nil
	} else {
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"StringEndsWith does not support %s function", f.Function.DebugString())))
	}
}
