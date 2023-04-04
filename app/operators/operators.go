package operators

import (
	"encoding/json"
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
	GetDBField(path []string, fieldName string) (interface{}, error)
	// GetListField(path []string) (interface{}, error)
}

// /////////////////////////////
// Operator
// /////////////////////////////
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
