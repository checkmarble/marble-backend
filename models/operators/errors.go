package operators

import "errors"

// Is used when a null value is read in a db field operator
var OperatorNullValueReadError = errors.New("Field read with null value")

// Is used when no rows are read in a db field operator
var OperatorNoRowsReadInDbError = errors.New("No rows read in db")

var OperatorDivisionByZeroError = errors.New("Division by zero")
