package ast

import (
	"github.com/cockroachdb/errors"
)

// ExecutionError represents an error that can occur during the execution of a rule
// It is used to communicate errors to the user
// If you need more information about the error, look at EvaluationErrorDto
// More info https://github.com/checkmarble/marble-backend/pull/526#discussion_r1537852952
type ExecutionError int

const (
	NoError        ExecutionError = 0
	DivisionByZero ExecutionError = 100
	NullFieldRead  ExecutionError = 200
	Unknown        ExecutionError = -1

	// legacy fields, rule executions are no longer created with those but old rules may still have them
	NoRowsRead           ExecutionError = 201
	PayloadFieldNotFound ExecutionError = 202
)

func (r ExecutionError) String() string {
	switch r {
	case DivisionByZero:
		return "A division by zero occurred in a rule"
	case NullFieldRead:
		return "A field read in a rule is null"
	case NoRowsRead:
		return "No rows were read from db in a rule"
	case PayloadFieldNotFound:
		return "A payload field was not found in a rule"
	case Unknown:
		return "Unknown error"
	}
	return ""
}

func AdaptExecutionError(err error) ExecutionError {
	switch {
	case err == nil:
		return NoError
	case errors.Is(err, ErrDivisionByZero):
		return DivisionByZero
	default:
		return Unknown
	}
}

var ExecutionAuthorizedErrors = []error{
	ErrDivisionByZero,
}

func IsAuthorizedError(err error) bool {
	for _, authorizedError := range ExecutionAuthorizedErrors {
		if errors.Is(err, authorizedError) {
			return true
		}
	}
	return false
}
