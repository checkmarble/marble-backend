package scoring

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func createRuleset(thresholds []int) models.ScoringRuleset {
	return models.ScoringRuleset{
		Thresholds: thresholds,
	}
}

func createEval(modifier, floor int) models.ScoringEvaluation {
	return models.ScoringEvaluation{
		Modifier: modifier,
		Floor:    floor,
	}
}

func TestInternalScoreToScore(t *testing.T) {
	uc := ScoringUsecase{}

	r := createRuleset([]int{10, 20, 30, 40})
	e := createEval(25, 0)

	assert.Equal(t, 2, uc.internalScoreToScore(r, e))

	r = createRuleset([]int{10, 20, 30, 40})
	e = createEval(25, 4)

	assert.Equal(t, 4, uc.internalScoreToScore(r, e))

	r = createRuleset([]int{10, 20, 30, 40})
	e = createEval(5, 0)

	assert.Equal(t, 1, uc.internalScoreToScore(r, e))

	r = createRuleset([]int{10, 20, 30, 40})
	e = createEval(5, 3)

	assert.Equal(t, 3, uc.internalScoreToScore(r, e))

	r = createRuleset([]int{10, 20, 30, 40})
	e = createEval(45, 0)

	assert.Equal(t, 5, uc.internalScoreToScore(r, e))
}
