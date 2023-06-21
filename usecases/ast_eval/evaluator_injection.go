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
	inject.AddEvaluator(ast.FUNC_ADD, evaluate.Arithmetic{Function: ast.FUNC_ADD})
	inject.AddEvaluator(ast.FUNC_SUBTRACT, evaluate.Arithmetic{Function: ast.FUNC_SUBTRACT})
	inject.AddEvaluator(ast.FUNC_MULTIPLY, evaluate.Arithmetic{Function: ast.FUNC_MULTIPLY})
	inject.AddEvaluator(ast.FUNC_DIVIDE, evaluate.Arithmetic{Function: ast.FUNC_DIVIDE})
	inject.AddEvaluator(ast.FUNC_GREATER, evaluate.Comparison{Function: ast.FUNC_GREATER})
	inject.AddEvaluator(ast.FUNC_LESS, evaluate.Comparison{Function: ast.FUNC_LESS})
	inject.AddEvaluator(ast.FUNC_EQUAL, evaluate.Comparison{Function: ast.FUNC_EQUAL})
	inject.AddEvaluator(ast.FUNC_NOT, evaluate.Comparison{Function: ast.FUNC_NOT})
	return inject
}
