package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
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

func TestReviewHandlersRejectMalformedBodyWithBadRequest(t *testing.T) {
	handlers := map[string]func(usecases.Usecases) func(*gin.Context){
		"single match": handleUpdateContinuousScreeningMatchStatus,
		"batch":        handleReviewContinuousScreeningMatches,
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"matches":[`))
			c.Request.Header.Set("Content-Type", "application/json")

			handler(usecases.Usecases{})(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestReviewContinuousScreeningMatchesStopsWhenContextIsDone(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// A valid reviewer and review: a dispatched update gets past validation into the empty usecases
	// and fails the test, so a 408 here means nothing was dispatched.
	creds := models.Credentials{ActorIdentity: models.Identity{UserId: "0b5e8f3a-2c4d-4e6f-8a9b-1c2d3e4f5a6b"}}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), utils.ContextKeyCredentials, creds))
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"matches":[{"match_id":"6f2b1c1e-8d3a-4c5e-9b7f-0a1d2e3f4a5b","status":"no_hit"}]}`,
	)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")

	handleReviewContinuousScreeningMatches(usecases.Usecases{})(c)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.NotContains(t, w.Body.String(), "results")
}
