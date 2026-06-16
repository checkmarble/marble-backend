package agent_dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreeningHitSuggestions_RoundTrip(t *testing.T) {
	original := ScreeningHitSuggestions{
		ScreeningHitSuggestionV1{
			MatchId:    "019eca91-4bf5-738e-b8a0-1d89dbf8e219",
			EntityId:   "Q1317",
			Confidence: models.ScreeningHitConfidenceProbableFalsePositive,
			Reason:     "names do not match",
			Version:    VersionScreeningHitSuggestionV1,
			CreatedAt:  time.Date(2026, 6, 15, 11, 35, 6, 0, time.UTC),
		},
		ScreeningHitSuggestionV1{
			MatchId:    "019eca91-4c1e-70c7-896c-4a6c562a760a",
			EntityId:   "Q2821441",
			Confidence: models.ScreeningHitConfidenceInconclusive,
			Reason:     "not enough information",
			Version:    VersionScreeningHitSuggestionV1,
			CreatedAt:  time.Date(2026, 6, 15, 11, 35, 7, 0, time.UTC),
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ScreeningHitSuggestions
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original, decoded)
}

// Decoding the flat, per-element format that is actually persisted inside CaseReviewContext
// (each object carries its own "version") must dispatch to the right concrete type.
func TestScreeningHitSuggestions_UnmarshalPersistedFormat(t *testing.T) {
	const payload = `[
		{
			"match_id": "019eca91-4bf5-738e-b8a0-1d89dbf8e219",
			"entity_id": "Q1317",
			"confidence": "probable_false_positive",
			"reason": "names do not match",
			"version": "v1",
			"created_at": "2026-06-15T11:35:06.711318Z"
		}
	]`

	var decoded ScreeningHitSuggestions
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))

	require.Len(t, decoded, 1)
	v1, ok := decoded[0].(ScreeningHitSuggestionV1)
	require.True(t, ok, "element should decode to ScreeningHitSuggestionV1")
	assert.Equal(t, "Q1317", v1.EntityId)
	assert.Equal(t, models.ScreeningHitConfidenceProbableFalsePositive, v1.Confidence)
	assert.Equal(t, VersionScreeningHitSuggestionV1, decoded[0].GetVersion())
}

func TestScreeningHitSuggestions_UnmarshalNullAndEmpty(t *testing.T) {
	var fromNull ScreeningHitSuggestions
	require.NoError(t, json.Unmarshal([]byte("null"), &fromNull))
	assert.Nil(t, fromNull)

	var fromEmpty ScreeningHitSuggestions
	require.NoError(t, json.Unmarshal([]byte("[]"), &fromEmpty))
	assert.Empty(t, fromEmpty)
}

func TestScreeningHitSuggestions_UnmarshalUnknownVersion(t *testing.T) {
	const payload = `[{"version": "v999"}]`

	var decoded ScreeningHitSuggestions
	err := json.Unmarshal([]byte(payload), &decoded)
	assert.Error(t, err)
}
