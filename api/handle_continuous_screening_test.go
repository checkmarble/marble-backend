package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchReviewStatusCode(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{nil, http.StatusOK},
		{errors.Wrap(models.BadParameterError, "invalid status"), http.StatusBadRequest},
		{errors.Wrap(models.UnprocessableEntityError, "continuous screening match is not pending review"), http.StatusUnprocessableEntity},
		{errors.Wrap(models.ForbiddenError, "no access"), http.StatusForbidden},
		{errors.Wrap(models.NotFoundError, "no match"), http.StatusNotFound},
		{errors.New("database"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, matchReviewStatusCode(tt.err))
	}
}

func TestContinuousScreeningMatchesReviewDtoReadsEachMatchUpdate(t *testing.T) {
	var payload dto.ContinuousScreeningMatchesReviewDto
	require.NoError(t, json.Unmarshal(
		[]byte(`{"matches":[{"match_id":"m1","status":"no_hit","comment":"Different birth date"}]}`),
		&payload,
	))
	require.Len(t, payload.Matches, 1)
	assert.Equal(t, "m1", payload.Matches[0].MatchId)
	assert.Equal(t, "no_hit", payload.Matches[0].Status)
	require.NotNil(t, payload.Matches[0].Comment)
	assert.Equal(t, "Different birth date", *payload.Matches[0].Comment)
}
