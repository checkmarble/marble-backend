package evaluate

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Comparison struct {
	Function ast.Function
}

func NewComparison(f ast.Function) Comparison {
	return Comparison{
		Function: f,
	}
}

func (f Comparison) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	leftFloat, rightFloat, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64)
	if len(errs) == 0 {
		return MakeEvaluateResult(f.comparisonFloatFunction(leftFloat, rightFloat))
	}

	leftTime, rightTime, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToTime);
	if len(errs) == 0 {
		return MakeEvaluateResult(f.comparisonTimeFunction(leftTime, rightTime))
	}
	return MakeEvaluateError(fmt.Errorf("all arguments must be an integer, a float or a time"))
}

func (f Comparison) comparisonFloatFunction(l, r float64) (bool, error) {
	switch f.Function {
	case ast.FUNC_GREATER:
		return l > r, nil
	case ast.FUNC_GREATER_OR_EQUAL:
		return l >= r, nil
	case ast.FUNC_LESS:
		return l < r, nil
	case ast.FUNC_LESS_OR_EQUAL:
		return l <= r, nil
	default:
		return false, fmt.Errorf("Comparison does not support %s function", f.Function.DebugString())
	}
}

func (f Comparison) comparisonTimeFunction(l, r time.Time) (bool, error) {
	switch f.Function {
	case ast.FUNC_GREATER:
		return l.After(r), nil
	case ast.FUNC_GREATER_OR_EQUAL:
		return l.After(r) || l.Equal(r), nil
	case ast.FUNC_LESS:
		return l.Before(r), nil
	case ast.FUNC_LESS_OR_EQUAL:
		return l.Before(r) || l.Equal(r), nil
	default:
		return false, fmt.Errorf("Comparison does not support %s function", f.Function.DebugString())
	}
}
