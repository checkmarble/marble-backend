package ast_eval

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

func EvalAst(node ast.Node) (any, error) {

	operands := ast.Operands{}

	// Eval children
	var err error
	operands.Args, err = utils.MapErr(node.Children, EvalAst)
	if err != nil {
		return nil, err
	}

	// Eval named children
	operands.NamedArgs, err = utils.MapMapErr(node.NamedChildren, EvalAst)
	if err != nil {
		return nil, err
	}

	return resolve(node, operands)
}

func resolve(node ast.Node, operands ast.Operands) (any, error) {
	if node.Constant != nil {
		return node.Constant, nil
	}

	switch node.Function {
	case ast.FUNC_PLUS:
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) + r.(int), nil
	case ast.FUNC_MINUS:
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) - r.(int), nil
	case ast.FUNC_GREATER:
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) > r.(int), nil
	case ast.FUNC_LESS:
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) < r.(int), nil
	case ast.FUNC_EQUAL_INT:
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) == r.(int), nil
	case ast.FUNC_DB_ACCESS:
		// Use operands.NamedArgs["tableName"]
		return 42, nil
	default:
		return nil, fmt.Errorf("resolve not implemented for function '%s'", node.Function.DebugString())
	}
}
