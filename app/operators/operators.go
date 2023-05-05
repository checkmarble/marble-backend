package operators

import (
	"encoding/json"
	"errors"
	"time"
)

///////////////////////////////
// Init function
///////////////////////////////
// Used to generate a map[operator ID]func() Operator
// This allows registering each operator's unique ID and constructor

var operatorFromType = make(map[string]func() Operator)

type DataAccessor interface {
	GetPayloadField(fieldName string) (interface{}, error)
	GetDbField(path []string, fieldName string) (interface{}, error)
	// GetListField(path []string) (interface{}, error)
	ValidateDbFieldReadConsistency(path []string, fieldName string) error
}

var ErrDbReadInconsistentWithDataModel = errors.New("Data model inconsistent with path or field name read from DB")

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
	// 	Needs() ([]APIField, []DBField, []DBVariable, []List)

	// DescribeForFront() []byte // JSON representation of each operator > STRONG LINK BT app and API

	// We need operators to Marshall & Unmarshall to JSON themselves
	json.Marshaler
	json.Unmarshaler

	// Self-print
	Print() string
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
	Eval(dataAccessor DataAccessor) (float64, error)
}

type OperatorBool interface {
	Operator
	Eval(dataAccessor DataAccessor) (bool, error)
}

type OperatorDate interface {
	Operator
	Eval(dataAccessor DataAccessor) (time.Time, error)
}

type OperatorString interface {
	Operator
	Eval(dataAccessor DataAccessor) (string, error)
}
