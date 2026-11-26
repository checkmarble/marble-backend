package dto

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptScreeningMatchUpdateInputDto_OptionalComment(t *testing.T) {
	for _, status := range []string{"no_hit", "confirmed_hit"} {
		for _, tc := range []struct {
			name    string
			comment *string
		}{
			{name: "omitted"},
			{name: "empty", comment: new("")},
			{name: "whitespace", comment: new(" \t\n\u00a0")},
			{name: "nonempty", comment: new("  Different birth date.\n")},
		} {
			t.Run(status+"/"+tc.name, func(t *testing.T) {
				update, err := AdaptScreeningMatchUpdateInputDto("match-id", models.UserId("reviewer-id"), ScreeningMatchUpdateDto{
					Status: status, Comment: tc.comment, Whitelist: true,
				})
				require.NoError(t, err)
				assert.Equal(t, "match-id", update.MatchId)
				require.NotNil(t, update.ReviewerId)
				assert.Equal(t, models.UserId("reviewer-id"), *update.ReviewerId)
				assert.Equal(t, status, update.Status.String())
				assert.True(t, update.Whitelist)
				if tc.name != "nonempty" {
					assert.Nil(t, update.Comment)
					return
				}
				assert.Equal(t, &models.ScreeningMatchComment{
					MatchId: "match-id", CommenterId: models.UserId("reviewer-id"), Comment: "  Different birth date.\n",
				}, update.Comment)
			})
		}
	}
}

func TestAdaptScreeningMatchCommentInputDto(t *testing.T) {
	t.Run("rejects a blank comment", func(t *testing.T) {
		_, err := AdaptScreeningMatchCommentInputDto("match-id", ScreeningMatchCommentDto{Comment: " \n"})

		assert.ErrorIs(t, err, models.BadParameterError)
	})

	t.Run("plain comment has no tentative review", func(t *testing.T) {
		comment, err := AdaptScreeningMatchCommentInputDto("match-id", ScreeningMatchCommentDto{Comment: "hello"})

		require.NoError(t, err)
		assert.Nil(t, comment.TentativeReview)
	})

	t.Run("keeps the status and ignores the client inbox", func(t *testing.T) {
		comment, err := AdaptScreeningMatchCommentInputDto("match-id", ScreeningMatchCommentDto{
			Comment:         "hello",
			TentativeReview: &ScreeningMatchTentativeReviewDto{Status: "no_hit", InboxId: uuid.NewString()},
		})

		require.NoError(t, err)
		require.NotNil(t, comment.TentativeReview)
		assert.Equal(t, "no_hit", comment.TentativeReview.Status)
		assert.Equal(t, uuid.Nil, comment.TentativeReview.InboxId)
	})
}

func TestAdaptScreeningMatchCommentDto_TentativeReview(t *testing.T) {
	inboxId := uuid.New()

	out := AdaptScreeningMatchCommentDto(models.ScreeningMatchComment{
		Comment:         "hello",
		TentativeReview: &models.ScreeningMatchTentativeReview{Status: "confirmed_hit", InboxId: inboxId},
	})

	require.NotNil(t, out.TentativeReview)
	assert.Equal(t, "confirmed_hit", out.TentativeReview.Status)
	assert.Equal(t, inboxId.String(), out.TentativeReview.InboxId)
	assert.Nil(t, AdaptScreeningMatchCommentDto(models.ScreeningMatchComment{}).TentativeReview)
}

func TestAdaptScreeningMatchComments_SeparatesTentativeReview(t *testing.T) {
	inboxId := uuid.New()
	superseded := time.Now()

	comments, review := adaptScreeningMatchComments([]models.ScreeningMatchComment{
		{Id: "plain", Comment: "a note"},
		{
			Id:              "old",
			Comment:         "earlier opinion",
			TentativeReview: &models.ScreeningMatchTentativeReview{Status: "no_hit", InboxId: inboxId, SupersededAt: &superseded},
		},
		{
			Id:              "review",
			CommenterId:     "user-id",
			Comment:         "looks like a hit",
			TentativeReview: &models.ScreeningMatchTentativeReview{Status: "confirmed_hit", InboxId: inboxId},
		},
	})

	require.Len(t, comments, 2)
	assert.Equal(t, "plain", comments[0].Id)
	assert.Nil(t, comments[0].TentativeReview)
	assert.Equal(t, "old", comments[1].Id)
	require.NotNil(t, comments[1].TentativeReview)
	assert.Equal(t, "no_hit", comments[1].TentativeReview.Status)
	assert.Equal(t, &superseded, comments[1].TentativeReview.SupersededAt)
	require.NotNil(t, review)
	assert.Equal(t, "review", review.Id)
	assert.Equal(t, "confirmed_hit", review.Status)
	assert.Equal(t, inboxId.String(), review.InboxId)
	assert.Equal(t, "looks like a hit", review.Comment)

	comments, review = adaptScreeningMatchComments(nil)

	assert.NotNil(t, comments)
	assert.Empty(t, comments)
	assert.Nil(t, review)
}
