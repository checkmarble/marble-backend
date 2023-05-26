package models

import (
	"fmt"
)

type Operands struct {
	Args      []AnyValue
	NamedArgs map[string]AnyValue
}

func (o *Operands) AddNamedArgs(name string, value AnyValue) {
	if o.NamedArgs == nil {
		o.NamedArgs = make(map[string]AnyValue)
	}
	o.NamedArgs[name] = value
}

func (o *Operands) LeftAndRight() (AnyValue, AnyValue, error) {
	if len(o.Args) >= 2 {
		return o.Args[0], o.Args[1], nil
	}
	return nil, nil, fmt.Errorf("not enough operands")
}

func EvalAst(node *ASTNode) (AnyValue, error) {

	operands := Operands{}

	// Eval each child
	for _, child := range node.Children {
		result, err := EvalAst(child)
		if err != nil {
			return nil, err
		}
		operands.Args = append(operands.Args, result)
	}

	// Eval named child:
	if node.NamedChildren != nil {
		for name, child := range node.NamedChildren {
			result, err := EvalAst(child)
			if err != nil {
				return nil, err
			}
			operands.AddNamedArgs(name, result)
		}
	}

	return resolve(node, operands)
}

func resolve(node *ASTNode, operands Operands) (AnyValue, error) {
	if node.Constant != nil {
		return node.Constant, nil
	}

	switch node.FuncName {
	case "+":
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) + r.(int), nil
	case ">":
		l, r, err := operands.LeftAndRight()
		if err != nil {
			return nil, err
		}
		return l.(int) > r.(int), nil
	case "DatabaseAccess":
		// Use operands.NamedArgs["tableName"]
		return 42, nil
	default:
		return nil, fmt.Errorf("unknown FuncName: '%s'", node.FuncName)
	}
}
