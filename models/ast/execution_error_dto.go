package ast

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

// ExecutionError represents an error that can occur during the execution of a rule
// It is used to communicate errors to the user
// If you need more information about the error, look at EvaluationErrorDto
// More info https://github.com/checkmarble/marble-backend/pull/526#discussion_r1537852952
type ExecutionError int

const (
	NoError              ExecutionError = 0
	DivisionByZero       ExecutionError = 100
	NullFieldRead        ExecutionError = 200
	NoRowsRead           ExecutionError = 201
	PayloadFieldNotFound ExecutionError = 202
	Unknown              ExecutionError = -1
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
	case errors.Is(err, ErrNullFieldRead):
		return NullFieldRead
	case errors.Is(err, ErrNoRowsRead):
		return NoRowsRead
	case errors.Is(err, ErrDivisionByZero):
		return DivisionByZero
	case errors.Is(err, ErrPayloadFieldNotFound):
		return PayloadFieldNotFound
	default:
		return Unknown
	}
}

func AdaptErrorCodeAsError(errCode ExecutionError) error {
	switch errCode {
	case NoError:
		return nil
	case NullFieldRead:
		return ErrNullFieldRead
	case NoRowsRead:
		return ErrNoRowsRead
	case DivisionByZero:
		return ErrDivisionByZero
	case PayloadFieldNotFound:
		return ErrPayloadFieldNotFound
	default:
		return fmt.Errorf("unknown error code")
	}
}
