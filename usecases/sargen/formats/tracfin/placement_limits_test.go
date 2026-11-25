package tracfin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlacementLimitsScaleWithSelectionAndAreCapped(t *testing.T) {
	small, big, huge := placementLimits(10), placementLimits(200), placementLimits(100000)
	assert.Equal(t, 35, small.MaxToolCalls)
	assert.Equal(t, 130, big.MaxToolCalls)
	assert.Greater(t, big.MaxTurns, small.MaxTurns)
	assert.Greater(t, big.TimeoutSeconds, small.TimeoutSeconds)
	assert.Equal(t, 300, huge.MaxToolCalls)
	assert.Equal(t, 150, huge.MaxTurns)
	assert.Equal(t, 600, huge.TimeoutSeconds)
}
