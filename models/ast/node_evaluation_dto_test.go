package ast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func encodeDecodeNodeEvaluation(t *testing.T, evaluation NodeEvaluation) NodeEvaluationDto {
	jsonData, err := json.Marshal(AdaptNodeEvaluationDto(evaluation))
	assert.NoError(t, err)

	var result NodeEvaluationDto
	err = json.Unmarshal(jsonData, &result)
	assert.NoError(t, err)

	return result
}

func TestAdaptAdaptNodeEvaluationDto_noerror(t *testing.T) {
	// evaluation succeded -> errors is encoded as en empty array
	result := encodeDecodeNodeEvaluation(t, NodeEvaluation{
		Errors: []error{},
	})

	assert.NotNil(t, result.Errors)
	assert.Len(t, result.Errors, 0)
}

func TestAdaptAdaptNodeEvaluationDto_noevaluation(t *testing.T) {
	// no evaluation -> errors is encoded as nil
	result := encodeDecodeNodeEvaluation(t, NodeEvaluation{
		Errors: nil,
	})
	assert.Empty(t, result.Errors)
}
