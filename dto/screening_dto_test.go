package dto

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
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
