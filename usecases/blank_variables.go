package usecases

import (
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
)

func addBlankVariableEvaluators(environment *ast_eval.AstEvaluationEnvironment, usecases *Usecases, organizationId string, returnFakeValue bool) {
	blankDbAccess := evaluate.BlankDatabaseAccess{
		OrganizationIdOfContext: organizationId,
		OrgTransactionFactory:   usecases.NewOrgTransactionFactory(),
		BlankDataReadRepository: usecases.Repositories.BlankDataReadRepository,
		ReturnFakeValue:         returnFakeValue,
		// Function:                specified below
	}

	environment.AddEvaluator(ast.FUNC_BLANK_FIRST_TRANSACTION_DATE, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_FIRST_TRANSACTION_DATE))
	environment.AddEvaluator(ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT))
	environment.AddEvaluator(ast.FUNC_BLANK_SEPA_OUT_FRACTIONATED, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_SEPA_OUT_FRACTIONATED))
	environment.AddEvaluator(ast.FUNC_BLANK_SEPA_NON_FR_IN_WINDOW, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_SEPA_NON_FR_IN_WINDOW))
	environment.AddEvaluator(ast.FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW))
	environment.AddEvaluator(ast.FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW, newBlankDbAccessWithFunction(blankDbAccess, ast.FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW))

}

func newBlankDbAccessWithFunction(dbAccess evaluate.BlankDatabaseAccess, function ast.Function) evaluate.BlankDatabaseAccess {
	dbAccess.Function = function
	return dbAccess
}
