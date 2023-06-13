package operators

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// /////////////////////////////
// Init function
// /////////////////////////////
// Used to generate a map[operator ID]func() Operator
// This allows registering each operator's unique ID and constructor
var operatorFromType = make(map[string]func() Operator)

type DataAccessor interface {
	GetPayloadField(fieldName string) (interface{}, error)
	GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error)
	GetDbHandle() (db *pgxpool.Pool, schema string, err error)
	GetTriggerObjectName() string
	ExecutionType() string
}

var (
	ErrDbReadInconsistentWithDataModel = errors.New("Data model inconsistent with path or field name read from DB")
	ErrEvaluatingInvalidOperator       = errors.New("Error evaluating invalid opereator")
)

// /////////////////////////////
// Operator
// /////////////////////////////

// Common serialized operator structure:
// {
// 	 "type" : string, name (ID) of the operator
// 	 "staticData" : struct, static values of the operator (e.g. constant values, path for field to read...)
// 	 "children" : slice of operators, children of the operator. E.g. equivalent of "left" and "right" in a
// 				  binary operator. Number of elements in slice enforced at marshalling/unmarshalling time.
// }

type Operator interface {
	// We need operators to Marshall & Unmarshall to JSON themselves
	json.Marshaler
	json.Unmarshaler

	// Self-print
	String() string

	IsValid() bool
}

// Used to add and read the "type" kep anytime we marshal/unmarshal
type OperatorType struct {
	Type string `json:"type"`
}

// /////////////////////////////
// Specific types of operators depending on their return types
// /////////////////////////////
type OperatorFloat interface {
	Operator
	Eval(ctx context.Context, dataAccessor DataAccessor) (float64, error)
}

type OperatorBool interface {
	Operator
	Eval(ctx context.Context, dataAccessor DataAccessor) (bool, error)
}

type OperatorDate interface {
	Operator
	Eval(ctx context.Context, dataAccessor DataAccessor) (time.Time, error)
}

type OperatorString interface {
	Operator
	Eval(ctx context.Context, dataAccessor DataAccessor) (string, error)
}

type OperatorStringList interface {
	Operator
	Eval(ctx context.Context, dataAccessor DataAccessor) ([]string, error)
}
