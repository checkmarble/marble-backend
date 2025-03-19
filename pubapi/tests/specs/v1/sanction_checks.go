package v1

import (
	"net/http"
	"strings"
	"testing"

	api "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/gavv/httpexpect/v2"
)

func sanctionChecks(t *testing.T, e *httpexpect.Expect) {
	e.GET("/decisions/nouuid/sanction-checks").Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "UUID")
		})

	e.GET("/decisions/00000000-0000-0000-0000-000000000000/sanction-checks").Expect().
		Status(http.StatusNotFound).
		JSON().
		Object().Value("error").Object().Value("messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "does not exist")
		})

	{
		out := e.GET("/decisions/11111111-1111-1111-1111-111111111111/sanction-checks").Expect().
			Status(http.StatusOK).
			JSON().Path("$.data[0]").
			Object()

		out.HasValue("id", "11111111-1111-1111-1111-111111111111")
		out.HasValue("status", "in_review")
		out.Path("$.query.lorem").IsEqual("ipsum")

		matches := out.Value("matches").Array()

		matches.Length().IsEqual(3)
		matches.Value(0).Object().HasValue("id", "11111111-1111-1111-1111-111111111111")
		matches.Value(0).Object().HasValue("status", "pending")
		matches.Value(1).Object().HasValue("id", "22222222-2222-2222-2222-222222222222")
		matches.Value(1).Object().HasValue("status", "no_hit")
	}

	e.GET("/decisions/22222222-2222-2222-2222-222222222222/sanction-checks").Expect().
		Status(http.StatusNotFound).
		JSON().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "does not have a sanction check")
		})

	e.POST("/sanction-checks/matches/11111111-1111-1111-1111-111111111111").
		WithJSON(api.UpdateSanctionCheckMatchStatusParams{Status: "no_hit"}).
		Expect().
		Status(http.StatusOK).
		JSON().Path("$.data").Object().
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("status", "no_hit")

	e.POST("/sanction-checks/matches/11111111-1111-1111-1111-111111111111").
		WithJSON(api.UpdateSanctionCheckMatchStatusParams{Status: "invalid"}).
		Expect().
		Status(http.StatusBadRequest)

	e.POST("/sanction-checks/matches/22222222-2222-2222-2222-222222222222").
		WithJSON(api.UpdateSanctionCheckMatchStatusParams{Status: "no_hit"}).
		Expect().
		Status(http.StatusUnprocessableEntity).
		JSON().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "not pending review")
		})
}
