package ast

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdaptEvaluationErrorDto_an_error(t *testing.T) {
	err := fmt.Errorf("test error %w", ErrWrongNumberOfArgument)

	evaluationError := AdaptEvaluationErrorDto(err)

	assert.Equal(t, evaluationError.EvaluationError, "WRONG_NUMBER_OF_ARGUMENTS")
	assert.Equal(t, evaluationError.Message, "test error wrong number of arguments")
	assert.Nil(t, evaluationError.ArgumentIndex)
	assert.Nil(t, evaluationError.ArgumentName)
}

func TestAdaptEvaluationErrorDto_with_argument_error(t *testing.T) {
	err := fmt.Errorf("test error %w", NewArgumentError(666))

	evaluationError := AdaptEvaluationErrorDto(err)

	if assert.NotNil(t, evaluationError.ArgumentIndex) {
		assert.Equal(t, *evaluationError.ArgumentIndex, 666)
	}
	assert.Nil(t, evaluationError.ArgumentName)
}

func TestAdaptEvaluationErrorDto_with_named_argument_error(t *testing.T) {
	err := fmt.Errorf("test error %w", NewNamedArgumentError("diabolo"))

	evaluationError := AdaptEvaluationErrorDto(err)

	assert.Nil(t, evaluationError.ArgumentIndex)
	if assert.NotNil(t, evaluationError.ArgumentName) {
		assert.Equal(t, *evaluationError.ArgumentName, "diabolo")
	}
}
