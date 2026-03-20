package types

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrorMessagesPublicVsPrivate(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

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
		path := fmt.Sprintf("/%s", pure_utils.NewId().String())

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
		assert.NotContains(t, resp.Error.Code, "private")
		assert.NotContains(t, resp.Error.Messages, "private")
		assert.Contains(t, resp.Error.Messages, "public1")
		assert.Contains(t, resp.Error.Messages, "public2")
	}
}
