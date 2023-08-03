package usecases

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

func addBlankVariableEvaluators(environment *ast_eval.AstEvaluationEnvironment, usecases *Usecases, organizationId string, returnFakeValue bool) {
	environment.AddEvaluator(ast.FUNC_BLANK_FIRST_TRANSACTION_DATE, evaluate.NewBlankDatabaseAccess(
		usecases.NewOrgTransactionFactory(),
		usecases.Repositories.BlankDataReadRepository,
		organizationId,
		ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
		returnFakeValue,
	))
}
