package v1

import (
	"net/http"
	"strings"
	"testing"

	gdto "github.com/checkmarble/marble-backend/dto"
	api "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/gavv/httpexpect/v2"
	"github.com/h2non/gock"
)

func screenings(t *testing.T, e *httpexpect.Expect) {
	e.GET("/decisions/nouuid/screenings").Expect().
		Status(http.StatusBadRequest).
		JSON().
		Object().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "UUID")
		})

	e.GET("/decisions/00000000-0000-0000-0000-000000000000/screenings").Expect().
		Status(http.StatusNotFound).
		JSON().
		Object().Value("error").Object().Value("messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "does not exist")
		})

	{
		out := e.GET("/decisions/11111111-1111-1111-1111-111111111111/screenings").Expect().
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

	e.GET("/decisions/22222222-2222-2222-2222-222222222222/screenings").Expect().
		Status(http.StatusNotFound).
		JSON().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "does not have a screening")
		})

	gock.New("http://screening/match").
		Persist().
		Reply(http.StatusOK).
		BodyString(`{
				"responses": {
					"out":{
						"total": { "value": 1 },
						"results":[
							{
								"id":"ID",
								"referents": ["a", "b"],
								"match": true,
								"schema": "Person",
								"datasets": ["one", "two"],
								"properties": {
									"name": ["Bob Joe"]
								}
							},
							{
								"id":"ID",
								"referents": ["a", "b"],
								"match": false,
								"schema": "Person",
								"datasets": ["one", "two"],
								"properties": {
									"name": ["Not A Match"]
								}
							}
						]
					}
				}
			}`)

	{
		matches := e.POST("/screening/search").
			WithJSON(gdto.RefineQueryDto{Thing: &gdto.RefineQueryBase{Name: "test"}}).
			Expect().
			Status(http.StatusOK).
			JSON().Path("$.data").Array()

		matches.Length().IsEqual(1)

		match := matches.Value(0).Object()
		match.HasValue("id", "ID")
		match.HasValue("referents", []string{"a", "b"})
		match.HasValue("datasets", []string{"one", "two"})
		match.Path("$.properties").Object().HasValue("name", []string{"Bob Joe"})
	}

	{
		matches := e.POST("/screening/11111111-1111-1111-1111-111111111111/search").
			WithJSON(gdto.RefineQueryDto{Thing: &gdto.RefineQueryBase{Name: "test"}}).
			Expect().
			Status(http.StatusOK).
			JSON().Path("$.data").Array()

		matches.Length().IsEqual(1)

		match := matches.Value(0).Object()
		match.HasValue("id", "ID")
		match.HasValue("referents", []string{"a", "b"})
		match.HasValue("datasets", []string{"one", "two"})
		match.Path("$.properties").Object().HasValue("name", []string{"Bob Joe"})
	}

	e.POST("/screening/matches/11111111-1111-1111-1111-111111111111").
		WithJSON(api.UpdateScreeningMatchStatusParams{Status: "no_hit"}).
		Expect().
		Status(http.StatusOK).
		JSON().Path("$.data").Object().
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("status", "no_hit")

	e.POST("/screening/matches/11111111-1111-1111-1111-111111111111").
		WithJSON(api.UpdateScreeningMatchStatusParams{Status: "invalid"}).
		Expect().
		Status(http.StatusBadRequest)

	e.POST("/screening/matches/22222222-2222-2222-2222-222222222222").
		WithJSON(api.UpdateScreeningMatchStatusParams{Status: "no_hit"}).
		Expect().
		Status(http.StatusUnprocessableEntity).
		JSON().Path("$.error.messages").Array().
		Find(func(index int, value *httpexpect.Value) bool {
			return strings.Contains(value.String().Raw(), "not pending review")
		})

	{
		matches := e.POST("/screening/22222222-2222-2222-2222-222222222222/refine").
			WithJSON(gdto.RefineQueryDto{Thing: &gdto.RefineQueryBase{Name: "test"}}).
			Expect().
			Status(http.StatusOK).
			JSON().Path("$.data.matches").Array()

		matches.Length().IsEqual(1)

		match := matches.Value(0).Object().Path("$.payload").Object()
		match.HasValue("id", "ID")
		match.HasValue("referents", []string{"a", "b"})
		match.HasValue("datasets", []string{"one", "two"})
		match.Path("$.properties").Object().HasValue("name", []string{"Bob Joe"})

		out := e.GET("/decisions/11111111-1111-1111-1111-111111111111/screenings").Expect().
			Status(http.StatusOK).
			JSON().Path("$.data[1]").
			Object()

		out.HasValue("status", "in_review")

		matches = out.Value("matches").Array()
		matches.Length().IsEqual(1)

		match = matches.Value(0).Object().Path("$.payload").Object()
		match.HasValue("id", "ID")
	}
}
