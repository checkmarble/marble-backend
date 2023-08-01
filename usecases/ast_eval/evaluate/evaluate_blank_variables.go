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
		accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
		if err != nil {
			return nil, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_FIRST_TRANSACTION_DATE): error reading accountId from payload: %w", err)
		}
		return blank.getFirstTransactionDate(accountId)
	default:
		return nil, fmt.Errorf("BlankDatabaseAccess: value not found: %w", ErrRuntimeExpression)
	}
}

func (blank BlankDatabaseAccess) getFirstTransactionDate(accountId string) (any, error) {
	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (any, error) {
			return blank.BlankDataReadRepository.GetFirstTransactionTimestamp(tx, accountId)
		})
}
