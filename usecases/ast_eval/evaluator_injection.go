package ast_eval

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

type EvaluatorInjection struct {
	availableFunctions map[ast.Function]evaluate.Evaluator
}

func (inject *EvaluatorInjection) AddEvaluator(function ast.Function, evaluator evaluate.Evaluator) {
	if _, ok := inject.availableFunctions[function]; ok {
		panic(fmt.Errorf("function '%s' is already registered", function.DebugString()))
	}
	inject.availableFunctions[function] = evaluator
}

func (inject *EvaluatorInjection) GetEvaluator(function ast.Function) (evaluate.Evaluator, error) {
	if funcClass, ok := inject.availableFunctions[function]; ok {
		return funcClass, nil
	}
	return nil, fmt.Errorf("function '%s' is not available", function.DebugString())
}

func NewEvaluatorInjection() EvaluatorInjection {
	inject := EvaluatorInjection{
		availableFunctions: make(map[ast.Function]evaluate.Evaluator),
	}

	// add pure functions that to not rely on any context
	inject.AddEvaluator(ast.FUNC_ADD, evaluate.NewArithmetic(ast.FUNC_ADD))
	inject.AddEvaluator(ast.FUNC_SUBTRACT, evaluate.NewArithmetic(ast.FUNC_SUBTRACT))
	inject.AddEvaluator(ast.FUNC_MULTIPLY, evaluate.NewArithmetic(ast.FUNC_MULTIPLY))
	inject.AddEvaluator(ast.FUNC_DIVIDE, evaluate.NewArithmetic(ast.FUNC_DIVIDE))
	inject.AddEvaluator(ast.FUNC_GREATER, evaluate.NewComparison(ast.FUNC_GREATER))
	inject.AddEvaluator(ast.FUNC_LESS, evaluate.NewComparison(ast.FUNC_LESS))
	inject.AddEvaluator(ast.FUNC_EQUAL, evaluate.Equal{})
	inject.AddEvaluator(ast.FUNC_NOT, evaluate.Not{Function: ast.FUNC_NOT})
	inject.AddEvaluator(ast.FUNC_AND, evaluate.BooleanArithmetic{Function: ast.FUNC_AND})
	inject.AddEvaluator(ast.FUNC_OR, evaluate.BooleanArithmetic{Function: ast.FUNC_OR})
	inject.AddEvaluator(ast.FUNC_IS_IN_LIST, evaluate.NewStringInList(ast.FUNC_IS_IN_LIST))
	inject.AddEvaluator(ast.FUNC_IS_NOT_IN_LIST, evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST))
	return inject
}
