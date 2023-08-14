package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/org_transaction"
	"math"
	"time"
)

type sepaDirection int

const (
	sepaIn sepaDirection = iota
	sepaOut
)

type blankWindowFnArguments struct {
	ownerBusinessId string
	referenceTime   time.Time
	amountThreshold float64
	numberThreshold int
}

type windowFunctionParams struct {
	periodStart     time.Time
	windowDuration  time.Duration
	numberThreshold int
	amountThreshold float64
}

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
	case ast.FUNC_BLANK_SEPA_NON_FR_IN_WINDOW:
		return blank.severalSepaNonFrWindow(arguments, sepaIn)
	case ast.FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW:
		return blank.severalSepaNonFrWindow(arguments, sepaOut)
	case ast.FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW:
		return blank.fractionatedTransferReceived(arguments)
	default:
		return nil, fmt.Errorf("BlankDatabaseAccess: value not found: %w", models.ErrRuntimeExpression)
	}
}

func (blank BlankDatabaseAccess) getFirstTransactionDate(arguments ast.Arguments) (time.Time, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return time.Time{}, err
	}

	ownerBusinessId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_FIRST_TRANSACTION_DATE): error reading ownerBusinessId from arguments: %w", err)
	}

	if blank.ReturnFakeValue {
		return time.Now(), nil
	}

	return org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(tx repositories.Transaction) (time.Time, error) {
			return blank.BlankDataReadRepository.GetFirstTransactionTimestamp(tx, ownerBusinessId)
		})
}

func (blank BlankDatabaseAccess) sumTransactionsAmount(arguments ast.Arguments) (float64, error) {
	if err := verifyNumberOfArguments(blank.Function, arguments.Args, 1); err != nil {
		return 0, err
	}

	ownerBusinessId, err := adaptArgumentToString(blank.Function, arguments.Args[0])
	if err != nil {
		return 0, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT): error reading ownerBusinessId from arguments: %w", err)
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
			return blank.BlankDataReadRepository.SumTransactionsAmount(tx, ownerBusinessId, direction, createdFrom, createdTo)
		})
}

func (blank BlankDatabaseAccess) sepaOutFractionated(arguments ast.Arguments) (bool, error) {
	args, err := adaptArgumentsBlankWindowVariable(arguments, blank.Function)
	if err != nil {
		return false, err
	}

	if blank.ReturnFakeValue {
		return true, nil
	}

	windowDuration := time.Duration(24) * time.Hour
	periodDuration := time.Duration(24*7) * time.Hour

	transactionsToRetrievePeriodStart := args.referenceTime.Add(-windowDuration - periodDuration)
	transactionsToCheckPeriodStart := args.referenceTime.Add(-periodDuration)
	txSlice, err := org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(dbTx repositories.Transaction) ([]map[string]any, error) {
			return blank.BlankDataReadRepository.RetrieveTransactions(
				dbTx,
				map[string]any{"owner_business_id": args.ownerBusinessId, "direction": "Debit", "type": "virement sortant", "cleared": true},
				transactionsToRetrievePeriodStart,
			)
		})
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_OUT_FRACTIONATED): error reading transactions from DB: %w", err)
	}

	return executeWindowFunctionSearch(txSlice,
		windowFunctionParams{
			periodStart:     transactionsToCheckPeriodStart,
			windowDuration:  windowDuration,
			numberThreshold: args.numberThreshold,
			amountThreshold: args.amountThreshold},
		walkWindowFindFractionated,
	)
}

func (blank BlankDatabaseAccess) severalSepaNonFrWindow(arguments ast.Arguments, direction sepaDirection) (bool, error) {
	args, err := adaptArgumentsBlankWindowVariable(arguments, blank.Function)
	if err != nil {
		return false, err
	}

	if blank.ReturnFakeValue {
		return true, nil
	}

	var windowDuration time.Duration
	periodDuration := time.Duration(24*7) * time.Hour

	filters := map[string]any{"owner_business_id": args.ownerBusinessId, "cleared": true}
	if direction == sepaIn {
		windowDuration = time.Duration(24) * time.Hour
		filters["direction"] = "Credit"
		filters["type"] = "virement entrant"
	}
	if direction == sepaOut {
		windowDuration = time.Duration(2*24) * time.Hour
		filters["direction"] = "Debit"
		filters["type"] = "virement sortant"
	}

	transactionsToRetrievePeriodStart := args.referenceTime.Add(-windowDuration - periodDuration)
	transactionsToCheckPeriodStart := args.referenceTime.Add(-periodDuration)
	txSlice, err := org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(dbTx repositories.Transaction) ([]map[string]any, error) {
			return blank.BlankDataReadRepository.RetrieveTransactions(dbTx, filters, transactionsToRetrievePeriodStart)
		})
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_SEPA_NON_FR_WINDOW): error reading transactions from DB: %w", err)
	}

	return executeWindowFunctionSearch(
		txSlice,
		windowFunctionParams{
			periodStart:     transactionsToCheckPeriodStart,
			windowDuration:  windowDuration,
			numberThreshold: args.numberThreshold,
			amountThreshold: args.amountThreshold},
		walkWindowFindMultipleNonFrTransfers,
	)
}

func (blank BlankDatabaseAccess) fractionatedTransferReceived(arguments ast.Arguments) (bool, error) {
	args, err := adaptArgumentsBlankWindowVariable(arguments, blank.Function)
	if err != nil {
		return false, err
	}

	if blank.ReturnFakeValue {
		return true, nil
	}

	windowDuration := time.Duration(5) * time.Minute
	periodDuration := time.Duration(24*7) * time.Hour

	filters := map[string]any{"owner_business_id": args.ownerBusinessId, "cleared": true, "direction": "Credit", "type": "virement entrant"}

	transactionsToRetrievePeriodStart := args.referenceTime.Add(-windowDuration - periodDuration)
	transactionsToCheckPeriodStart := args.referenceTime.Add(-periodDuration)
	txSlice, err := org_transaction.InOrganizationSchema(
		blank.OrgTransactionFactory,
		blank.OrganizationIdOfContext,
		func(dbTx repositories.Transaction) ([]map[string]any, error) {
			return blank.BlankDataReadRepository.RetrieveTransactions(dbTx, filters, transactionsToRetrievePeriodStart)
		})
	if err != nil {
		return false, fmt.Errorf("BlankDatabaseAccess (FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW): error reading transactions from DB: %w", err)
	}

	return executeWindowFunctionSearch(
		txSlice,
		windowFunctionParams{
			periodStart:     transactionsToCheckPeriodStart,
			windowDuration:  windowDuration,
			numberThreshold: args.numberThreshold,
			amountThreshold: args.amountThreshold},
		walkWindowFindFractionated,
	)
}

func executeWindowFunctionSearch(
	transactions []map[string]any,
	params windowFunctionParams,
	fn func([]map[string]any, windowFunctionParams) (bool, error),
) (bool, error) {
	for i := range transactions {
		// only check the transactions that are in the period to check (not the buffer added on top that is only necessary
		// to compute the aggregates)
		if windowStart, ok := transactions[i]["created_at"].(time.Time); !ok {
			return false, fmt.Errorf("BlankDatabaseAccess: error reading created_at from transaction")
		} else if windowStart.Before(params.periodStart) {
			break
		}
		if found, err := fn(transactions[i:], params); err != nil {
			return false, fmt.Errorf("BlankDatabaseAccess: error walking window: %w", err)
		} else if found {
			return true, nil
		}
	}
	return false, nil
}

func walkWindowFindFractionated(transactions []map[string]any, params windowFunctionParams) (bool, error) {
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
	timeWindowStart := timeWindowEnd.Add(-params.windowDuration)

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
	if nbSameIban >= params.numberThreshold && totalSameIban >= params.amountThreshold {
		return true, nil
	}
	return false, nil
}

func walkWindowFindMultipleNonFrTransfers(transactions []map[string]any, params windowFunctionParams) (bool, error) {
	// The implementation assumes that the transactions are sorted by date, descending
	if len(transactions) == 0 {
		return false, nil
	}
	iban, ok := transactions[0]["counterparty_iban"].(string)
	if !ok {
		return false, fmt.Errorf("walkWindowFindMultipleNonFrTransfers: error reading iban from transaction")
	}
	if iban[:2] == "FR" {
		return false, nil
	}

	timeWindowEnd, ok := transactions[0]["created_at"].(time.Time)
	if !ok {
		return false, fmt.Errorf("walkWindowFindMultipleNonFrTransfers: error reading created_at from transaction")
	}
	timeWindowStart := timeWindowEnd.Add(-params.windowDuration)

	var totalNonFrIban float64 = 0
	nbNonFrIban := 0
	for i := 0; i < len(transactions); i++ {
		thisCreatedAt, ok := transactions[i]["created_at"].(time.Time)
		if !ok {
			return false, fmt.Errorf("walkWindowFindMultipleNonFrTransfers: error reading created_at from transaction")
		}
		if thisCreatedAt.Before(timeWindowStart) {
			break // outside of for loop because of type assertion
		}
		thisIban, ok := transactions[i]["counterparty_iban"].(string)
		if !ok {
			return false, fmt.Errorf("walkWindowFindMultipleNonFrTransfers: error reading iban from transaction")
		}
		if thisIban[:2] != "FR" {
			amount, ok := transactions[i]["txn_amount"].(float64)
			if !ok {
				return false, fmt.Errorf("walkWindowFindMultipleNonFrTransfers: error reading txn_amount from transaction")
			}
			totalNonFrIban += amount
			nbNonFrIban++
		}
	}
	if nbNonFrIban >= params.numberThreshold && totalNonFrIban >= params.amountThreshold {
		return true, nil
	}
	return false, nil
}

func adaptArgumentsBlankWindowVariable(arguments ast.Arguments, fn ast.Function) (blankWindowFnArguments, error) {
	if err := verifyNumberOfArguments(fn, arguments.Args, 2); err != nil {
		return blankWindowFnArguments{}, err
	}

	ownerBusinessId, err := adaptArgumentToString(fn, arguments.Args[0])
	if err != nil {
		return blankWindowFnArguments{}, fmt.Errorf("BlankDatabaseAccess: error reading ownerBusinessId from arguments: %w", err)
	}
	referenceTime, err := adaptArgumentToTime(fn, arguments.Args[1])
	if err != nil {
		return blankWindowFnArguments{}, fmt.Errorf("BlankDatabaseAccess: error reading time from arguments: %w", err)
	}
	amountThreshold, err := promoteArgumentToFloat64(fn, arguments.NamedArgs["amountThreshold"])
	if err != nil {
		return blankWindowFnArguments{}, fmt.Errorf("BlankDatabaseAccess: error reading amountThreshold from named arguments: %w", err)
	}
	// NB: this is a float64, not an int64 because of json decoding
	numberThresholdFloat, err := promoteArgumentToFloat64(fn, arguments.NamedArgs["numberThreshold"])
	if err != nil {
		return blankWindowFnArguments{}, fmt.Errorf("BlankDatabaseAccess: error reading numberThreshold from named arguments: %w", err)
	}
	numberThreshold := int(math.Round(numberThresholdFloat))

	return blankWindowFnArguments{
		ownerBusinessId: ownerBusinessId,
		referenceTime:   referenceTime,
		amountThreshold: amountThreshold,
		numberThreshold: numberThreshold,
	}, nil
}
