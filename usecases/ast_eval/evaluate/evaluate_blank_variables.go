package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/org_transaction"
)

type BlankDatabaseAccess struct {
	OrganizationIdOfContext string
	OrgTransactionFactory   org_transaction.Factory
	BlankDataReadRepository repositories.BlankDataReadRepository
	Function                ast.Function
}

func NewBlankDatabaseAccess(
	otf org_transaction.Factory,
	bdrr repositories.BlankDataReadRepository,
	organizationIdOfContext string,
	f ast.Function,
) BlankDatabaseAccess {
	return BlankDatabaseAccess{
		OrganizationIdOfContext: organizationIdOfContext,
		OrgTransactionFactory:   otf,
		BlankDataReadRepository: bdrr,
		Function:                f,
	}
}

func (blank BlankDatabaseAccess) Evaluate(arguments ast.Arguments) (any, error) {

	switch blank.Function {
	case ast.FUNC_BLANK_FIRST_TRANSACTION_DATE:
		return blank.getFirstTransactionDate(arguments)
	case ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT:
		return blank.sumTransactionsAmount(arguments)
	default:
		return nil, fmt.Errorf("BlankDatabaseAccess: value not found: %w", ErrRuntimeExpression)
	}
}

func (blank BlankDatabaseAccess) getFirstTransactionDate(arguments ast.Arguments) (any, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return nil, err
	}

	accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_FIRST_TRANSACTION_DATE): error reading accountId from payload: %w", err)
	}

	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (any, error) {
			return blank.BlankDataReadRepository.GetFirstTransactionTimestamp(tx, accountId)
		})
}

func (blank BlankDatabaseAccess) sumTransactionsAmount(arguments ast.Arguments) (any, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return nil, err
	}

	accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading accountId from payload: %w", err)
	}
	direction, err := adaptArgumentToString(blank.Function, arguments.NamedArgs["direction"])
	if err != nil {
		return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading direction from payload: %w", err)
	}
	createdFrom, err := adaptArgumentToTime(blank.Function, arguments.NamedArgs["created_from"])
	if err != nil {
		return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading created_from from payload: %w", err)
	}
	createdTo, err := adaptArgumentToTime(blank.Function, arguments.NamedArgs["created_to"])
	if err != nil {
		return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading created_to from payload: %w", err)
	}
	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (any, error) {
			return blank.BlankDataReadRepository.SumTransactionsAmount(tx, accountId, direction, createdFrom, createdTo)
		})
}
