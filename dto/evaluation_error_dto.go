package dto

import (
	"errors"
	"marble/marble-backend/models/ast"
)

type EvaluationErrorCodeDto string

const (
	UNEXPECTED_ERROR          EvaluationErrorCodeDto = "UNEXPECTED_ERROR"
	UNKNOWN_FUNCTION          EvaluationErrorCodeDto = "UNKNOWN_FUNCTION"
	WRONG_NUMBER_OF_ARGUMENTS EvaluationErrorCodeDto = "WRONG_NUMBER_OF_ARGUMENTS"
	MISSING_NAMED_ARGUMENT    EvaluationErrorCodeDto = "MISSING_NAMED_ARGUMENT"
)

type EvaluationErrorDto struct {
	EvaluationError EvaluationErrorCodeDto `json:"error"`
	Message         string                 `json:"message"`
}

type errorAndCode struct {
	err  error
	code EvaluationErrorCodeDto
}

var evaluationErrorDtoMap = []errorAndCode{
	{ast.ErrWrongNumberOfArgument, WRONG_NUMBER_OF_ARGUMENTS},
	{ast.ErrMissingNamedArgument, MISSING_NAMED_ARGUMENT},
	{ast.ErrUnknownFunction, UNKNOWN_FUNCTION},
}

func AdaptEvaluationErrorDto(err error) EvaluationErrorDto {

	if err == nil {
		return EvaluationErrorDto{
			EvaluationError: UNEXPECTED_ERROR,
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
		EvaluationError: UNEXPECTED_ERROR,
		Message:         err.Error(),
	}
}
