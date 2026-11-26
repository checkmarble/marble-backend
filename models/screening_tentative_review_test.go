package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreeningMatchTentativeReviewValidate(t *testing.T) {
	for _, status := range []string{"confirmed_hit", "no_hit"} {
		assert.NoError(t, ScreeningMatchTentativeReview{Status: status}.Validate(), status)
	}

	for _, status := range []string{"", "pending", "skipped", "nonsense"} {
		assert.ErrorIs(t, ScreeningMatchTentativeReview{Status: status}.Validate(), BadParameterError, status)
	}
}

func TestScreeningMatchCommentToCaseEvent(t *testing.T) {
	orgId := uuid.New()

	event := ScreeningMatchComment{
		MatchId:         "match-id",
		CommenterId:     "user-id",
		Comment:         "looks wrong",
		TentativeReview: &ScreeningMatchTentativeReview{Status: "no_hit"},
	}.ToCaseEvent(ScreeningMatchResourceType, orgId, "case-id")

	assert.Equal(t, ScreeningMatchCommented, event.EventType)
	assert.Equal(t, orgId, event.OrgId)
	assert.Equal(t, "case-id", event.CaseId)
	require.NotNil(t, event.UserId)
	assert.Equal(t, "user-id", *event.UserId)
	require.NotNil(t, event.ResourceType)
	assert.Equal(t, ScreeningMatchResourceType, *event.ResourceType)
	require.NotNil(t, event.ResourceId)
	assert.Equal(t, "match-id", *event.ResourceId)
	require.NotNil(t, event.NewValue)
	assert.Equal(t, "no_hit", *event.NewValue)
	require.NotNil(t, event.AdditionalNote)
	assert.Equal(t, "looks wrong", *event.AdditionalNote)
}
