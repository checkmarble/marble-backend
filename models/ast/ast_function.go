package ast

import (
	"fmt"
)

type Function int

var FuncOperators = []Function{
	FUNC_ADD,
	FUNC_SUBTRACT,
	FUNC_MULTIPLY,
	FUNC_DIVIDE,
	FUNC_GREATER,
	FUNC_GREATER_OR_EQUAL,
	FUNC_LESS,
	FUNC_LESS_OR_EQUAL,
	FUNC_EQUAL,
	FUNC_NOT_EQUAL,
	FUNC_IS_IN_LIST,
	FUNC_IS_NOT_IN_LIST,
	FUNC_STRING_CONTAINS,
	FUNC_STRING_NOT_CONTAIN,
	FUNC_CONTAINS_ANY,
	FUNC_CONTAINS_NONE,
}

const (
	FUNC_CONSTANT Function = iota
	FUNC_ADD
	FUNC_SUBTRACT
	FUNC_MULTIPLY
	FUNC_DIVIDE
	FUNC_GREATER
	FUNC_GREATER_OR_EQUAL
	FUNC_LESS
	FUNC_LESS_OR_EQUAL
	FUNC_EQUAL
	FUNC_NOT_EQUAL
	FUNC_NOT
	FUNC_AND
	FUNC_OR
	FUNC_TIME_ADD
	FUNC_TIME_NOW
	FUNC_PARSE_TIME
	FUNC_PAYLOAD
	FUNC_DB_ACCESS
	FUNC_CUSTOM_LIST_ACCESS
	FUNC_IS_IN_LIST
	FUNC_IS_NOT_IN_LIST
	FUNC_STRING_CONTAINS
	FUNC_STRING_NOT_CONTAIN
	FUNC_CONTAINS_ANY
	FUNC_CONTAINS_NONE
	FUNC_AGGREGATOR
	FUNC_LIST
	FUNC_FILTER
	FUNC_BLANK_FIRST_TRANSACTION_DATE
	FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT
	FUNC_BLANK_SEPA_OUT_FRACTIONATED
	FUNC_BLANK_SEPA_NON_FR_IN_WINDOW
	FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW
	FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW
	FUNC_UNDEFINED Function = -1
	FUNC_UNKNOWN   Function = -2
)

type FuncAttributes struct {
	DebugName         string
	AstName           string
	NumberOfArguments int
	NamedArguments    []string
}

// If number of arguments -1 the function can take any number of arguments
var FuncAttributesMap = map[Function]FuncAttributes{
	FUNC_UNDEFINED: {
		DebugName: "UNDEFINED",
		AstName:   "Undefined",
	},
	FUNC_CONSTANT: {
		DebugName: "CONSTANT",
		AstName:   "",
	},
	FUNC_ADD: {
		DebugName:         "FUNC_ADD",
		AstName:           "+",
		NumberOfArguments: 2,
	},
	FUNC_SUBTRACT: {
		DebugName:         "FUNC_SUBTRACT",
		AstName:           "-",
		NumberOfArguments: 2,
	},
	FUNC_MULTIPLY: {
		DebugName:         "FUNC_MULTIPLY",
		AstName:           "*",
		NumberOfArguments: 2,
	},
	FUNC_DIVIDE: {
		DebugName:         "FUNC_DIVIDE",
		AstName:           "/",
		NumberOfArguments: 2,
	},
	FUNC_GREATER: {
		DebugName:         "FUNC_GREATER",
		AstName:           ">",
		NumberOfArguments: 2,
	},
	FUNC_GREATER_OR_EQUAL: {
		DebugName:         "FUNC_GREATER_OR_EQUAL",
		AstName:           ">=",
		NumberOfArguments: 2,
	},
	FUNC_LESS: {
		DebugName:         "FUNC_LESS",
		AstName:           "<",
		NumberOfArguments: 2,
	},
	FUNC_LESS_OR_EQUAL: {
		DebugName:         "FUNC_LESS_OR_EQUAL",
		AstName:           "<=",
		NumberOfArguments: 2,
	},
	FUNC_EQUAL: {
		DebugName:         "FUNC_EQUAL",
		AstName:           "=",
		NumberOfArguments: 2,
	},
	FUNC_NOT_EQUAL: {
		DebugName:         "FUNC_NOT_EQUAL",
		AstName:           "≠",
		NumberOfArguments: 2,
	},
	FUNC_NOT: {
		DebugName:         "FUNC_NOT",
		AstName:           "Not",
		NumberOfArguments: 1,
	},
	FUNC_AND: {
		DebugName:         "FUNC_AND",
		AstName:           "And",
		NumberOfArguments: -1,
	},
	FUNC_OR: {
		DebugName:         "FUNC_OR",
		AstName:           "Or",
		NumberOfArguments: -1,
	},
	FUNC_TIME_ADD: {
		DebugName:         "FUNC_TIME_ADD",
		AstName:           "TimeAdd",
		NumberOfArguments: 3,
		NamedArguments:    []string{"timestampField", "duration", "sign"},
	},
	FUNC_TIME_NOW: {
		DebugName:         "FUNC_TIME_NOW",
		AstName:           "TimeNow",
		NumberOfArguments: 0,
	},
	FUNC_PARSE_TIME: {
		DebugName:         "FUNC_PARSE_TIME",
		AstName:           "ParseTime",
		NumberOfArguments: 1,
	},
	FUNC_PAYLOAD: {
		DebugName:         "FUNC_PAYLOAD",
		AstName:           "Payload",
		NumberOfArguments: 1,
	},
	FUNC_DB_ACCESS:          AttributeFuncDbAccess.FuncAttributes,
	FUNC_CUSTOM_LIST_ACCESS: AttributeFuncCustomListAccess.FuncAttributes,
	FUNC_IS_IN_LIST: {
		DebugName:         "FUNC_IS_IN_LIST",
		AstName:           "IsInList",
		NumberOfArguments: 2,
	},
	FUNC_IS_NOT_IN_LIST: {
		DebugName:         "FUNC_IS_NOT_IN_LIST",
		AstName:           "IsNotInList",
		NumberOfArguments: 2,
	},
	FUNC_STRING_CONTAINS: {
		DebugName:         "FUNC_STRING_CONTAINS",
		AstName:           "StringContains",
		NumberOfArguments: 2,
	},
	FUNC_STRING_NOT_CONTAIN: {
		DebugName:         "FUNC_STRING_NOT_CONTAIN",
		AstName:           "StringNotContain",
		NumberOfArguments: 2,
	},
	FUNC_CONTAINS_ANY: {
		DebugName:         "FUNC_CONTAINS_ANY",
		AstName:           "ContainsAnyOf",
		NumberOfArguments: 2,
	},
	FUNC_CONTAINS_NONE: {
		DebugName:         "FUNC_CONTAINS_NONE",
		AstName:           "ContainsNoneOf",
		NumberOfArguments: 2,
	},
	FUNC_AGGREGATOR: FuncAggregatorAttributes,
	FUNC_LIST: {
		DebugName: "FUNC_LIST",
		AstName:   "List",
	},
	FUNC_FILTER: FuncFilterAttributes,
	FUNC_BLANK_FIRST_TRANSACTION_DATE: {
		DebugName:         "FUNC_BLANK_FIRST_TRANSACTION_DATE",
		AstName:           "BlankFirstTransactionDate",
		NumberOfArguments: 1,
	},
	FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT: {
		DebugName:         "FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT",
		AstName:           "BlankSumTransactionsAmount",
		NumberOfArguments: 1,
		// Or pass them as a single map ? To be discussed.
		NamedArguments: []string{"direction", "created_from", "created_to"},
	},
	FUNC_BLANK_SEPA_OUT_FRACTIONATED: {
		DebugName:         "FUNC_BLANK_SEPA_OUT_FRACTIONATED",
		AstName:           "BlankSepaOutFractionated",
		NumberOfArguments: 1,
		NamedArguments:    []string{"amountThreshold", "numberThreshold"},
	},
	FUNC_BLANK_SEPA_NON_FR_IN_WINDOW: {
		DebugName:         "FUNC_BLANK_SEPA_NON_FR_IN_WINDOW",
		AstName:           "BlankSepaNonFrInWindow",
		NumberOfArguments: 1,
		NamedArguments:    []string{"amountThreshold", "numberThreshold"},
	},
	FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW: {
		DebugName:         "FUNC_BLANK_SEPA_NON_FR_OUT_WINDOW",
		AstName:           "BlankSepaNonFrOutWindow",
		NumberOfArguments: 1,
		NamedArguments:    []string{"amountThreshold", "numberThreshold"},
	},
	FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW: {
		DebugName:         "FUNC_BLANK_QUICK_FRACTIONATED_TRANSFERS_RECEIVED_WINDOW",
		AstName:           "BlankQuickFractionatedTransfersReceivedWindow",
		NumberOfArguments: 1,
		NamedArguments:    []string{"amountThreshold", "numberThreshold"},
	},
}

func (f Function) Attributes() (FuncAttributes, error) {

	if attributes, ok := FuncAttributesMap[f]; ok {
		return attributes, nil
	}

	unknown := fmt.Sprintf("Unknown function: %v", f)
	return FuncAttributes{
		DebugName: unknown,
		AstName:   unknown,
	}, fmt.Errorf(unknown)
}

func (f Function) DebugString() string {
	attributes, _ := f.Attributes()
	return attributes.DebugName
}

// ======= Constant =======

func NewNodeConstant(value any) Node {
	return Node{Function: FUNC_CONSTANT, Constant: value}
}

// ======= DbAccess =======

var AttributeFuncDbAccess = struct {
	FuncAttributes
	ArgumentTableName string
	ArgumentFieldName string
	ArgumentPathName  string
}{
	FuncAttributes: FuncAttributes{
		DebugName: "FUNC_DB_ACCESS",
		AstName:   "DatabaseAccess",
		NamedArguments: []string{
			"tableName", "fieldName", "path",
		},
	},
	ArgumentTableName: "tableName",
	ArgumentFieldName: "fieldName",
	ArgumentPathName:  "path",
}

func NewNodeDatabaseAccess(tableName string, fieldName string, path []string) Node {
	return Node{Function: FUNC_DB_ACCESS}.
		AddNamedChild(AttributeFuncDbAccess.ArgumentTableName, NewNodeConstant(tableName)).
		AddNamedChild(AttributeFuncDbAccess.ArgumentFieldName, NewNodeConstant(fieldName)).
		AddNamedChild(AttributeFuncDbAccess.ArgumentPathName, NewNodeConstant(path))
}
