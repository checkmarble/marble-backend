package dto

import (
	"errors"

	"github.com/checkmarble/marble-backend/models/ast"
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
	{ast.ErrUndefinedFunction, "UNDEFINED_FUNCTION"},
	{ast.ErrWrongNumberOfArgument, "WRONG_NUMBER_OF_ARGUMENTS"},
	{ast.ErrMissingNamedArgument, "MISSING_NAMED_ARGUMENT"},
	{ast.ErrArgumentMustBeIntOrFloat, "ARGUMENTS_MUST_BE_INT_OR_FLOAT"},
	{ast.ErrArgumentMustBeIntFloatOrTime, "ARGUMENTS_MUST_BE_INT_FLOAT_OR_TIME"},
	{ast.ErrArgumentMustBeInt, "ARGUMENT_MUST_BE_INTEGER"},
	{ast.ErrArgumentMustBeString, "ARGUMENT_MUST_BE_STRING"},
	{ast.ErrArgumentMustBeBool, "ARGUMENT_MUST_BE_BOOLEAN"},
	{ast.ErrArgumentMustBeList, "ARGUMENT_MUST_BE_LIST"},
	{ast.ErrArgumentCantBeConvertedToDuration, "ARGUMENT_MUST_BE_CONVERTIBLE_TO_DURATION"},
	{ast.ErrArgumentCantBeTime, "ARGUMENT_MUST_BE_TIME"},
	{ast.ErrArgumentRequired, "ARGUMENT_REQUIRED"},
	{ast.ErrArgumentInvalidType, "ARGUMENT_INVALID_TYPE"},
	{ast.ErrListNotFound, "LIST_NOT_FOUND"},
	{ast.ErrDatabaseAccessNotFound, "DATABASE_ACCESS_NOT_FOUND"},
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
	var argumentError ast.ArgumentError
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
