package v1

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func cases(_ *testing.T, e *httpexpect.Expect) {
	cases := e.GET("/cases").Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array()

	cases.Length().IsEqual(2)

	cases.Value(0).Object().
		HasValue("id", "10000000-0000-0000-0000-000000000000").
		HasValue("name", "Case name")

	limitedCases := e.GET("/cases").WithQuery("limit", 1).Expect().
		Status(http.StatusOK).
		JSON().
		Object()

	limitedCases.Path("$.data").Array().Length().IsEqual(1)
	limitedCases.Path("$.pagination").Object().
		HasValue("has_more", true).
		HasValue("next_page_id", "10000000-0000-0000-0000-000000000000")

	cas := e.GET("/cases/10000000-0000-0000-0000-000000000000").Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Object()

	cas.HasValue("id", "10000000-0000-0000-0000-000000000000").
		HasValue("name", "Case name").
		HasValue("status", "closed").
		HasValue("outcome", "unset").
		HasValue("created_at", "2025-09-01T12:34:56.120Z")

	cas.Value("assignee").Object().
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("name", "Bob Example")
}
