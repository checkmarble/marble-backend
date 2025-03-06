package pubapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestErrorMessagesPublicVsPrivate(t *testing.T) {
	g := gin.New()

	tts := []struct {
		err            error
		expectedStatus int
	}{
		{io.EOF, http.StatusBadRequest},
		{models.NotFoundError, http.StatusNotFound},
		{ErrFeatureDisabled, http.StatusPaymentRequired},
		{ErrFeatureDisabled, http.StatusPaymentRequired},
		{ErrNotConfigured, http.StatusNotImplemented},
		{errors.New("private"), http.StatusInternalServerError},
	}

	for _, tt := range tts {
		path := fmt.Sprintf("/%s", uuid.NewString())

		g.GET(path, func(c *gin.Context) {
			err := errors.WithDetail(
				errors.WithDetail(
					errors.Wrap(tt.err, "private"),
					"public1",
				),
				"public2",
			)

			NewErrorResponse().WithError(err).Serve(c)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)

		g.ServeHTTP(w, r)

		var resp baseErrorResponse

		assert.Equal(t, tt.expectedStatus, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotContains(t, resp.Error.Message, "private")
		assert.NotContains(t, resp.Error.Details, "private")
		assert.Contains(t, resp.Error.Details, "public1")
		assert.Contains(t, resp.Error.Details, "public2")
	}
}
