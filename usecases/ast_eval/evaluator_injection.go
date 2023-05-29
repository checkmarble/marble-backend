package ast_eval

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

type EvaluatorInjectionImpl struct {
	availableFunctions map[ast.Function]evaluate.Evaluator
}

func (inject *EvaluatorInjectionImpl) AddEvaluator(function ast.Function, evaluator evaluate.Evaluator) {
	if _, ok := inject.availableFunctions[function]; ok {
		panic(fmt.Errorf("function '%s' is already registered", function.DebugString()))
	}
	inject.availableFunctions[function] = evaluator
}

func (inject *EvaluatorInjectionImpl) GetEvaluator(function ast.Function) (evaluate.Evaluator, error) {
	if funcClass, ok := inject.availableFunctions[function]; ok {
		return funcClass, nil
	}
	return nil, fmt.Errorf("function '%s' is not available", function.DebugString())
}

func NewEvaluatorInjection() EvaluatorInjectionImpl {
	inject := EvaluatorInjectionImpl{
		availableFunctions: make(map[ast.Function]evaluate.Evaluator),
	}

	// add pure functions that to not rely on any context
	inject.AddEvaluator(ast.FUNC_PLUS, evaluate.Arithmetic{Function: ast.FUNC_PLUS})
	inject.AddEvaluator(ast.FUNC_MINUS, evaluate.Arithmetic{Function: ast.FUNC_MINUS})
	inject.AddEvaluator(ast.FUNC_GREATER, evaluate.Comparison{Function: ast.FUNC_GREATER})
	inject.AddEvaluator(ast.FUNC_LESS, evaluate.Comparison{Function: ast.FUNC_LESS})
	inject.AddEvaluator(ast.FUNC_EQUAL, evaluate.Comparison{Function: ast.FUNC_EQUAL})
	return inject
}
