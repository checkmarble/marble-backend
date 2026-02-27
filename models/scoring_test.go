package models

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsOverriden_NilScore(t *testing.T) {
	var s *ScoringScore

	assert.False(t, s.IsOverriden())
}

func TestIsOverriden_NotOverride(t *testing.T) {
	s := &ScoringScore{Source: ScoreSourceRuleset}

	assert.False(t, s.IsOverriden())
}

func TestIsOverriden_Override_NoStaleAt(t *testing.T) {
	s := &ScoringScore{Source: ScoreSourceOverride, StaleAt: nil}

	assert.True(t, s.IsOverriden())
}

func TestIsOverriden_Override_FutureStaleAt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		staleAt := time.Now().Add(time.Hour)

		s := &ScoringScore{
			Source:  ScoreSourceOverride,
			StaleAt: &staleAt,
		}

		assert.True(t, s.IsOverriden())
	})
}

func TestIsOverriden_Override_PastStaleAt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		staleAt := time.Now().Add(-time.Hour)

		s := &ScoringScore{
			Source:  ScoreSourceOverride,
			StaleAt: &staleAt,
		}

		assert.False(t, s.IsOverriden())
	})
}

func TestIsStale_NilScore(t *testing.T) {
	var s *ScoringScore

	assert.True(t, s.IsStale(time.Hour))
}

func TestIsStale_Override_NeverStale(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		createdAt := time.Now().Add(-24 * time.Hour)

		s := &ScoringScore{
			Source:    ScoreSourceOverride,
			CreatedAt: createdAt,
		}

		assert.False(t, s.IsStale(time.Minute))
	})
}

func TestIsStale_Override_Fresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		createdAt := time.Now().Add(-24 * time.Hour)
		staleAt := time.Now().Add(time.Hour)

		s := &ScoringScore{
			Source:    ScoreSourceOverride,
			CreatedAt: createdAt,
			StaleAt:   &staleAt,
		}

		assert.False(t, s.IsStale(time.Minute))
	})
}

func TestIsStale_Override_ExpiredStaleAt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		staleAt := now.Add(-time.Hour)

		s := &ScoringScore{
			Source:    ScoreSourceOverride,
			StaleAt:   &staleAt,
			CreatedAt: now.Add(-2 * time.Hour),
		}

		assert.True(t, s.IsStale(time.Minute))
	})
}

func TestIsStale_Fresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &ScoringScore{
			Source:    ScoreSourceRuleset,
			CreatedAt: time.Now().Add(-time.Minute),
		}

		assert.False(t, s.IsStale(time.Hour))
	})
}

func TestIsStale_Old(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &ScoringScore{
			Source:    ScoreSourceRuleset,
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}
		assert.True(t, s.IsStale(time.Hour))
	})
}

func TestScoreSourceFrom(t *testing.T) {
	cases := []struct {
		input    string
		expected ScoreSource
	}{
		{"ruleset", ScoreSourceRuleset},
		{"override", ScoreSourceOverride},
		{"", ScoreSourceUnknown},
		{"something_else", ScoreSourceUnknown},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.expected, ScoreSourceFrom(tc.input), "input: %s", tc.input)
	}
}

func TestScoreRulesetStatusFrom(t *testing.T) {
	cases := []struct {
		input    string
		expected ScoreRulesetStatus
	}{
		{"draft", ScoreRulesetDraft},
		{"committed", ScoreRulesetCommitted},
		{"", ScoreRulesetUnknown},
		{"something_else", ScoreRulesetUnknown},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.expected, ScoreRulesetStatusFrom(tc.input), "input: %s", tc.input)
	}
}
