package ast_eval

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
)

type AstEvaluationEnvironment struct {
	availableFunctions map[ast.Function]evaluate.Evaluator
}

func (environment *AstEvaluationEnvironment) AddEvaluator(function ast.Function, evaluator evaluate.Evaluator) {
	if _, ok := environment.availableFunctions[function]; ok {
		panic(fmt.Errorf("function '%s' is already registered", function.DebugString()))
	}
	environment.availableFunctions[function] = evaluator
}

func (environment *AstEvaluationEnvironment) GetEvaluator(function ast.Function) (evaluate.Evaluator, error) {
	if funcClass, ok := environment.availableFunctions[function]; ok {
		return funcClass, nil
	}
	return nil, fmt.Errorf("function '%s' is not available", function.DebugString())
}

func NewAstEvaluationEnvironment() AstEvaluationEnvironment {
	environment := AstEvaluationEnvironment{
		availableFunctions: make(map[ast.Function]evaluate.Evaluator),
	}

	// add pure functions that to not rely on any context
	environment.AddEvaluator(ast.FUNC_UNDEFINED, evaluate.Undefined{})
	environment.AddEvaluator(ast.FUNC_ADD, evaluate.NewArithmetic(ast.FUNC_ADD))
	environment.AddEvaluator(ast.FUNC_SUBTRACT, evaluate.NewArithmetic(ast.FUNC_SUBTRACT))
	environment.AddEvaluator(ast.FUNC_MULTIPLY, evaluate.NewArithmetic(ast.FUNC_MULTIPLY))
	environment.AddEvaluator(ast.FUNC_DIVIDE, evaluate.ArithmeticDivide{})
	environment.AddEvaluator(ast.FUNC_GREATER, evaluate.NewComparison(ast.FUNC_GREATER))
	environment.AddEvaluator(ast.FUNC_GREATER_OR_EQUAL, evaluate.NewComparison(ast.FUNC_GREATER_OR_EQUAL))
	environment.AddEvaluator(ast.FUNC_LESS, evaluate.NewComparison(ast.FUNC_LESS))
	environment.AddEvaluator(ast.FUNC_LESS_OR_EQUAL, evaluate.NewComparison(ast.FUNC_LESS_OR_EQUAL))
	environment.AddEvaluator(ast.FUNC_EQUAL, evaluate.Equal{})
	environment.AddEvaluator(ast.FUNC_NOT, evaluate.Not{})
	environment.AddEvaluator(ast.FUNC_AND, evaluate.BooleanArithmetic{Function: ast.FUNC_AND})
	environment.AddEvaluator(ast.FUNC_OR, evaluate.BooleanArithmetic{Function: ast.FUNC_OR})
	environment.AddEvaluator(ast.FUNC_IS_IN_LIST, evaluate.NewStringInList(ast.FUNC_IS_IN_LIST))
	environment.AddEvaluator(ast.FUNC_IS_NOT_IN_LIST, evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST))
	environment.AddEvaluator(ast.FUNC_ADD_TIME, evaluate.NewTimeArithmetic(ast.FUNC_ADD_TIME))
	environment.AddEvaluator(ast.FUNC_TIME_NOW, evaluate.NewTimeFunctions(ast.FUNC_TIME_NOW))
	environment.AddEvaluator(ast.FUNC_PARSE_TIME, evaluate.NewTimeFunctions(ast.FUNC_PARSE_TIME))
	environment.AddEvaluator(ast.FUNC_LIST, evaluate.List{})
	return environment
}
