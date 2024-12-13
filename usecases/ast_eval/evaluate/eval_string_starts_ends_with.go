package evaluate

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type StringStartsEndsWith struct {
	Function ast.Function
}

func NewStringStartsEndsWith(f ast.Function) StringStartsEndsWith {
	return StringStartsEndsWith{
		Function: f,
	}
}

func (f StringStartsEndsWith) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
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

	rightString, errString := adaptArgumentToString(rightAny)
	rightList, errList := adaptArgumentToListOfStrings(rightAny)

	if errString == nil {
		return startsEndsWithString(left, rightString, f.Function)
	} else if errList == nil {
		return startsEndsWithListOfStrings(left, rightList, f.Function)
	} else {
		return MakeEvaluateError(errors.Wrap(
			ast.ErrArgumentMustBeStringOrList,
			fmt.Sprintf("can't promote %v to string or []string", rightAny),
		))
	}
}

func startsEndsWithString(left string, right string, f ast.Function) (any, []error) {
	if f == ast.FUNC_STRING_STARTS_WITH {
		return strings.HasPrefix(strings.ToLower(left), strings.ToLower(right)), nil
	} else if f == ast.FUNC_STRING_ENDS_WITH {
		return strings.HasSuffix(strings.ToLower(left), strings.ToLower(right)), nil
	} else {
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"StringStartsWith does not support %s function", f.DebugString())))
	}
}

func startsEndsWithListOfStrings(left string, right []string, f ast.Function) (any, []error) {
	if f == ast.FUNC_STRING_STARTS_WITH {
		for _, r := range right {
			if strings.HasPrefix(strings.ToLower(left), strings.ToLower(r)) {
				return true, nil
			}
		}
		return false, nil
	} else if f == ast.FUNC_STRING_ENDS_WITH {
		for _, r := range right {
			if strings.HasSuffix(strings.ToLower(left), strings.ToLower(r)) {
				return true, nil
			}
		}
		return false, nil
	} else {
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"StringStartsWith does not support %s function", f.DebugString())))
	}
}
