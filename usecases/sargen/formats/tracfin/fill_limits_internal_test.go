package tracfin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFillLimitsFormulaAndHardMaxima(t *testing.T) {
	assert.Equal(t, 30, fillMaxTurnsCap)
	assert.Equal(t, 60, fillMaxToolCallsCap)
	assert.Equal(t, 420, fillMaxTimeoutSeconds)

	l := fillLimits(0)
	assert.Equal(t, 15, l.MaxTurns)
	assert.Equal(t, 30, l.MaxToolCalls)
	l = fillLimits(10)
	assert.Equal(t, 25, l.MaxTurns)
	assert.Equal(t, 50, l.MaxToolCalls)
	assert.Equal(t, 260, l.TimeoutSeconds)
	// Large inputs are capped at 30 turns and 60 tool calls.
	for _, n := range []int{15, 16, 100, 100000} {
		l = fillLimits(n)
		assert.Equal(t, 30, l.MaxTurns, "n=%d", n)
		assert.Equal(t, 60, l.MaxToolCalls, "n=%d", n)
	}
	assert.Equal(t, 420, fillLimits(100000).TimeoutSeconds, "the time is capped too")
	assert.Equal(t, 188, fillLimits(1).TimeoutSeconds)
}
