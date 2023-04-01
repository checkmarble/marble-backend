package operators

import (
	"encoding/json"
	"marble/marble-backend/app/dynamic_reading"
	"time"
)

///////////////////////////////
// Init function
///////////////////////////////
// Used to generate a map[operator ID]func() Operator
// This allows registering each operator's unique ID and constructor

var operatorFromType = make(map[string]func() Operator)

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
	Eval(context dynamic_reading.EvaluationContext) float64
}

type OperatorBool interface {
	Operator
	Eval(context dynamic_reading.EvaluationContext) bool
}

type OperatorDate interface {
	Operator
	Eval(context dynamic_reading.EvaluationContext) time.Time
}

type OperatorString interface {
	Operator
	Eval(context dynamic_reading.EvaluationContext) string
}
