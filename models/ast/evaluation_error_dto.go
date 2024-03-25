package ast

import (
	"errors"
)

type EvaluationErrorDto struct {
	EvaluationError string  `json:"error"`
	Message         string  `json:"message"`
	ArgumentIndex   *int    `json:"argument_index,omitempty"`
	ArgumentName    *string `json:"argument_name,omitempty"`
}

type errorAndCode struct {
	err  error
	code string
}

var evaluationErrorDtoMap = []errorAndCode{
	// Validation related errors
	{ErrUndefinedFunction, "UNDEFINED_FUNCTION"},
	{ErrWrongNumberOfArgument, "WRONG_NUMBER_OF_ARGUMENTS"},
	{ErrMissingNamedArgument, "MISSING_NAMED_ARGUMENT"},
	{ErrArgumentMustBeIntOrFloat, "ARGUMENTS_MUST_BE_INT_OR_FLOAT"},
	{ErrArgumentMustBeIntFloatOrTime, "ARGUMENTS_MUST_BE_INT_FLOAT_OR_TIME"},
	{ErrArgumentMustBeInt, "ARGUMENT_MUST_BE_INTEGER"},
	{ErrArgumentMustBeString, "ARGUMENT_MUST_BE_STRING"},
	{ErrArgumentMustBeBool, "ARGUMENT_MUST_BE_BOOLEAN"},
	{ErrArgumentMustBeList, "ARGUMENT_MUST_BE_LIST"},
	{ErrArgumentCantBeConvertedToDuration, "ARGUMENT_MUST_BE_CONVERTIBLE_TO_DURATION"},
	{ErrArgumentMustBeTime, "ARGUMENT_MUST_BE_TIME"},
	{ErrArgumentRequired, "ARGUMENT_REQUIRED"},
	{ErrArgumentInvalidType, "ARGUMENT_INVALID_TYPE"},
	{ErrListNotFound, "LIST_NOT_FOUND"},
	{ErrDatabaseAccessNotFound, "DATABASE_ACCESS_NOT_FOUND"},

	// Runtime execution related errors
	{ErrNullFieldRead, "NULL_FIELD_READ"},
	{ErrNoRowsRead, "NO_ROWS_READ"},
	{ErrDivisionByZero, "DIVISION_BY_ZERO"},
	{ErrPayloadFieldNotFound, "PAYLOAD_FIELD_NOT_FOUND"},
	{ErrRuntimeExpression, "RUNTIME_EXPRESSION_ERROR"}, // must be last, as it is the most generic error (and above runtime errors are wrapped in it)
}

func AdaptEvaluationErrorDto(err error) EvaluationErrorDto {
	if err == nil {
		return EvaluationErrorDto{
			EvaluationError: "UNEXPECTED_ERROR",
			Message:         "Internal Error: err is not supposed to be nil",
		}
	}

	dto := EvaluationErrorDto{
		Message: err.Error(),
	}

	// extract argument index or name fron err
	var argumentError ArgumentError
	if errors.As(err, &argumentError) {
		if argumentError.ArgumentIndex >= 0 {
			dto.ArgumentIndex = &argumentError.ArgumentIndex
		}
		if argumentError.ArgumentName != "" {
			dto.ArgumentName = &argumentError.ArgumentName
		}
	}

	// find the corresponding error code
	for _, errorAndCode := range evaluationErrorDtoMap {
		if errors.Is(err, errorAndCode.err) {
			dto.EvaluationError = errorAndCode.code
			return dto
		}
	}

	dto.EvaluationError = "UNEXPECTED_ERROR"
	return dto
}
