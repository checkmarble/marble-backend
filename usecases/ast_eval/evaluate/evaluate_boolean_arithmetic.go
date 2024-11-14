package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type BooleanArithmetic struct {
	Function ast.Function
}

func NewBooleanArithmetic(f ast.Function) BooleanArithmetic {
	return BooleanArithmetic{
		Function: f,
	}
}

func (f BooleanArithmetic) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if len(arguments.Args) < 1 {
		return MakeEvaluateError(errors.Wrap(
			ast.ErrWrongNumberOfArgument,
			"Boolean arithmetic expects at least 1 operand, got 0"))
	}

	switch f.Function {
	case ast.FUNC_AND:
		return MakeEvaluateResult(booleanArithmeticEvalAnd(arguments.Args))
	case ast.FUNC_OR:
		return MakeEvaluateResult(booleanArithmeticEvalOr(arguments.Args))
	default:
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"Boolean arithmetic does not support %s function", f.Function.DebugString())))
	}
}

// Case OR:
// - if any true: return true
// - if any nulls: return null
// - else (all false): return false
func booleanArithmeticEvalOr(args []any) (any, error) {
	nullFound := false
	for _, arg := range args {
		if arg == nil {
			nullFound = true
		} else {
			argBool, ok := arg.(bool)
			if !ok {
				return nil, errors.Wrap(ast.ErrArgumentMustBeBool,
					"Boolean arithmetic expects all arguments to be boolean")
			}
			if argBool {
				return true, nil
			}
		}
	}
	if nullFound {
		return nil, nil
	}
	return false, nil
}

// Case AND:
// - if any false: return false
// - if any null: return null
// - else (only true): return true
func booleanArithmeticEvalAnd(args []any) (any, error) {
	nullFound := false
	for _, arg := range args {
		if arg == nil {
			nullFound = true
		} else {
			argBool, ok := arg.(bool)
			if !ok {
				return nil, errors.Wrap(ast.ErrArgumentMustBeBool,
					"Boolean arithmetic expects all arguments to be boolean")
			}
			if !argBool {
				return false, nil
			}
		}
	}
	if nullFound {
		return nil, nil
	}
	return true, nil
}
