package v1

import (
	"net/http"
	"testing"

	api "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/gavv/httpexpect/v2"
)

func PublicApiV1(t *testing.T, e *httpexpect.Expect) {
	e.POST("/example").
		WithJSON(api.ExamplePayload{
			Age:    22,
			Email:  "test@example.com",
			IsNice: "true",
		}).
		Expect().
		Status(http.StatusOK)

	sanctionChecks(t, e)
	whitelists(t, e)
}
