package dto

import (
	"errors"
	"marble/marble-backend/models/ast"
)

type EvaluationErrorDto struct {
	EvaluationError string `json:"error"`
	Message         string `json:"message"`
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
	{ast.ErrArgumentMustBeInt, "ARGUMENT_MUST_BE_INTEGER"},
	{ast.ErrArgumentMustBeString, "ARGUMENT_MUST_BE_STRING"},
	{ast.ErrArgumentMustBeBool, "ARGUMENT_MUST_BE_BOOLEAN"},
	{ast.ErrArgumentMustBeList, "ARGUMENT_MUST_BE_LIST"},
	{ast.ErrArgumentCantBeConvertedToDuration, "ARGUMENT_MUST_BE_CONVERTIBLE_TO_DURATION"},
	{ast.ErrArgumentCantBeTime, "ARGUMENT_MUST_BE_TIME"},
}

func AdaptEvaluationErrorDto(err error) EvaluationErrorDto {

	if err == nil {
		return EvaluationErrorDto{
			EvaluationError: "UNEXPECTED_ERROR",
			Message:         "Internal Error: err is not supposed to be nil",
		}
	}

	for _, errorAndCode := range evaluationErrorDtoMap {
		if errors.Is(err, errorAndCode.err) {
			return EvaluationErrorDto{
				EvaluationError: errorAndCode.code,
				Message:         err.Error(),
			}
		}
	}

	return EvaluationErrorDto{
		EvaluationError: "UNEXPECTED_ERROR",
		Message:         err.Error(),
	}
}
