package dto

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdaptEvaluationErrorDto_an_error(t *testing.T) {

	err := fmt.Errorf("test error %w", ast.ErrWrongNumberOfArgument)

	evaluationError := AdaptEvaluationErrorDto(err)

	assert.Equal(t, evaluationError.EvaluationError, WRONG_NUMBER_OF_ARGUMENTS)
	assert.Equal(t, evaluationError.Message, "test error wrong number of arguments")
}
