package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/org_transaction"
	"math"
	"time"
)

type BlankDatabaseAccess struct {
	OrganizationIdOfContext string
	OrgTransactionFactory   org_transaction.Factory
	BlankDataReadRepository repositories.BlankDataReadRepository
	Function                ast.Function
	ReturnFakeValue         bool
}

func NewBlankDatabaseAccess(
	otf org_transaction.Factory,
	bdrr repositories.BlankDataReadRepository,
	organizationIdOfContext string,
	f ast.Function,
	fake bool,
) BlankDatabaseAccess {
	return BlankDatabaseAccess{
		OrganizationIdOfContext: organizationIdOfContext,
		OrgTransactionFactory:   otf,
		BlankDataReadRepository: bdrr,
		Function:                f,
		ReturnFakeValue:         fake,
	}
}

func (blank BlankDatabaseAccess) Evaluate(arguments ast.Arguments) (any, error) {

	switch blank.Function {
	case ast.FUNC_BLANK_FIRST_TRANSACTION_DATE:
		return blank.getFirstTransactionDate(arguments)
	case ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT:
		return blank.sumTransactionsAmount(arguments)
	case ast.FUNC_BLANK_SEPA_OUT_FRACTIONATED:
		return blank.sepaOutFractionated(arguments)
	default:
		return nil, fmt.Errorf("BlankDatabaseAccess: value not found: %w", ErrRuntimeExpression)
	}
}

func (blank BlankDatabaseAccess) getFirstTransactionDate(arguments ast.Arguments) (time.Time, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return time.Time{}, err
	}

	accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_FIRST_TRANSACTION_DATE): error reading accountId from arguments: %w", err)
	}

	if blank.ReturnFakeValue {
		return time.Now(), nil
	}

	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (time.Time, error) {
			return blank.BlankDataReadRepository.GetFirstTransactionTimestamp(tx, accountId)
		})
}

func (blank BlankDatabaseAccess) sumTransactionsAmount(arguments ast.Arguments) (float64, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return 0, err
	}

	accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return 0, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading accountId from arguments: %w", err)
	}
	direction, err := adaptArgumentToString(blank.Function, arguments.NamedArgs["direction"])
	if err != nil {
		return 0, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading direction from arguments: %w", err)
	}
	createdFrom, err := adaptArgumentToTime(blank.Function, arguments.NamedArgs["created_from"])
	if err != nil {
		return 0, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading created_from from arguments: %w", err)
	}
	createdTo, err := adaptArgumentToTime(blank.Function, arguments.NamedArgs["created_to"])
	if err != nil {
		return 0, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading created_to from arguments: %w", err)
	}

	if blank.ReturnFakeValue {
		return 1000, nil
	}

	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (float64, error) {
			return blank.BlankDataReadRepository.SumTransactionsAmount(tx, accountId, direction, createdFrom, createdTo)
		})
}

func (blank BlankDatabaseAccess) sepaOutFractionated(arguments ast.Arguments) (bool, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return false, err
	}

	accountId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading accountId from arguments: %w", err)
	}
	amountThreshold, err := promoteArgumentToFloat64(blank.Function, arguments.NamedArgs["amountThreshold"])
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading amountThreshold from named arguments: %w", err)
	}
	// TODO FIXME: this is a float64, not an int64 because of json decoding
	numberThresholdFloat, err := promoteArgumentToFloat64(blank.Function, arguments.NamedArgs["numberThreshold"])
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading numberThreshold from named arguments: %w", err)
	}
	numberThreshold := int(math.Round(numberThresholdFloat))
	nbDaysWindow := 1
	nbDaysPeriod := 7

	if blank.ReturnFakeValue {
		return true, nil
	}

	transactionsToRetrievePeriodStart := time.Now().AddDate(0, -nbDaysPeriod-nbDaysWindow, 0)
	transactinsToCheckPeriodStart := time.Now().AddDate(0, -nbDaysPeriod, 0)
	txSlice, err := org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(dbTx repositories.Transaction) ([]map[string]any, error) {
			return blank.BlankDataReadRepository.RetrieveTransactions(dbTx, accountId, transactionsToRetrievePeriodStart)
		})
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading transactions from DB: %w", err)
	}

	for i := range txSlice {
		// only check the transactions that are in the period to check (not the buffer added on top that is only necessary
		// to compute the aggregates)
		if windowStart, ok := txSlice[i]["created_at"].(time.Time); !ok {
			return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading created_at from transaction")
		} else if windowStart.Before(transactinsToCheckPeriodStart) {
			break
		}
		if found, err := walkWindowFindFractionated(txSlice[i:], numberThreshold, amountThreshold, nbDaysWindow); err != nil {
			return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error walking window: %w", err)
		} else if found {
			return true, nil
		}
	}

	return false, nil
}

func walkWindowFindFractionated(transactions []map[string]any, numberThreshold int, amountThreshold float64, nbDaysWindow int) (bool, error) {
	// The implementation assumes that the transactions are sorted by date, descending
	if len(transactions) == 0 {
		return false, nil
	}
	iban, ok := transactions[0]["counterparty_iban"].(string)
	if !ok {
		return false, fmt.Errorf("walkWindowFindFractionated: error reading iban from transaction")
	}
	timeWindowEnd, ok := transactions[0]["created_at"].(time.Time)
	if !ok {
		return false, fmt.Errorf("walkWindowFindFractionated: error reading created_at from transaction")
	}
	timeWindowStart := timeWindowEnd.AddDate(0, 0, -nbDaysWindow)

	var totalSameIban float64 = 0
	nbSameIban := 0
	for i := 0; i < len(transactions); i++ {
		thisCreatedAt, ok := transactions[i]["created_at"].(time.Time)
		if !ok {
			return false, fmt.Errorf("walkWindowFindFractionated: error reading created_at from transaction")
		}
		if thisCreatedAt.Before(timeWindowStart) {
			break // outside of for loop because of type assertion
		}
		thisIban, ok := transactions[i]["counterparty_iban"].(string)
		if !ok {
			return false, fmt.Errorf("walkWindowFindFractionated: error reading iban from transaction")
		}
		if iban == thisIban {
			amount, ok := transactions[i]["txn_amount"].(float64)
			if !ok {
				return false, fmt.Errorf("walkWindowFindFractionated: error reading txn_amount from transaction")
			}
			totalSameIban += amount
			nbSameIban++
		}
	}
	if nbSameIban >= numberThreshold && totalSameIban >= amountThreshold {
		return true, nil
	}
	return false, nil
}
