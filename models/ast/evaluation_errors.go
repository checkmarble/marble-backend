package ast

import "github.com/cockroachdb/errors"

var (
	// Validation related errors
	ErrUndefinedFunction                 = errors.New("undefined function")
	ErrWrongNumberOfArgument             = errors.New("wrong number of arguments")
	ErrMissingNamedArgument              = errors.New("missing named argument")
	ErrArgumentMustBeIntOrFloat          = errors.New("arguments must be an integer or a float")
	ErrArgumentMustBeIntFloatOrTime      = errors.New("all arguments must be an integer, a float or a time")
	ErrArgumentMustBeStringOrList        = errors.New("arguments must be string or list")
	ErrArgumentMustBeInt                 = errors.New("arguments must be an integer")
	ErrArgumentMustBeString              = errors.New("arguments must be a string")
	ErrArgumentMustBeBool                = errors.New("arguments must be a boolean")
	ErrArgumentMustBeList                = errors.New("arguments must be a list")
	ErrArgumentCantBeConvertedToDuration = errors.New("argument cant be converted to duration")
	ErrArgumentMustBeTime                = errors.New("argument must be a time")
	ErrArgumentRequired                  = errors.New("argument is required")
	ErrArgumentInvalidType               = errors.New("argument has an invalid type")
	ErrListNotFound                      = errors.New("list not found")
	ErrDatabaseAccessNotFound            = errors.New("database access not found")
	ErrFilterTableNotMatch               = errors.New("filters must be applied on the same table")

	// Runtime execution related errors
	ErrRuntimeExpression    = errors.New("expression runtime error")
	ErrNullFieldRead        = errors.Wrap(ErrRuntimeExpression, "Null field read")
	ErrNoRowsRead           = errors.Wrap(ErrRuntimeExpression, "No rows read")
	ErrDivisionByZero       = errors.Wrap(ErrRuntimeExpression, "Division by zero")
	ErrPayloadFieldNotFound = errors.Wrap(ErrRuntimeExpression, "Payload field not found")
)

var ExecutionAuthorizedErrors = []error{
	ErrNullFieldRead,
	ErrNoRowsRead,
	ErrDivisionByZero,
	ErrPayloadFieldNotFound,
}

func IsAuthorizedError(err error) bool {
	for _, authorizedError := range ExecutionAuthorizedErrors {
		if errors.Is(err, authorizedError) {
			return true
		}
	}
	return false
}
