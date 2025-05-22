package v1

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func decisions(t *testing.T, e *httpexpect.Expect) {
	ds := e.GET("/decisions").Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array()

	ds.Length().IsEqual(2)

	e.GET("/decisions/11111111-1111-1111-1111-111111111112").Expect().
		Status(http.StatusNotFound)

	d := e.GET("/decisions/11111111-1111-1111-1111-111111111111").Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Object()

	d.
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("outcome", "block_and_review").
		HasValue("review_status", "pending")

	d.Path("$.case").Object().HasValue("id", "00000000-0000-0000-0000-000000000000")
	d.Path("$.scenario").Object().HasValue("id", "11111111-1111-1111-1111-111111111111")

	rules := d.Path("$.rules").Array()

	rules.Length().IsEqual(2)
	rules.Value(0).Object().HasValue("name", "The Rule")
	rules.Value(0).Object().HasValue("outcome", "no_hit")
	rules.Value(0).Object().HasValue("score_modifier", 10)
	rules.Value(0).Object().NotContainsKey("error")
	rules.Value(1).Object().HasValue("name", "The Rule")
	rules.Value(1).Object().HasValue("outcome", "hit")
	rules.Value(1).Object().HasValue("score_modifier", 20)
	rules.Value(1).Object().Path("$.error").Object().
		HasValue("code", 100).
		HasValue("message", "A division by zero occurred in a rule")

	testDecisionFilters(t, e)
}

func testDecisionFilters(t *testing.T, e *httpexpect.Expect) {
	ids := []string{
		"00000000-0000-0000-0000-000000000000",
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	}

	type spec struct {
		key, value    string
		expectedCount int
		expectedId    string
	}

	tts := []spec{
		{"scenario_id", ids[1], 1, ids[1]},
		{"case_id", ids[0], 2, ""},
		{"outcome", "decline", 1, ids[2]},
		{"trigger_object_id", ids[1], 1, ids[1]},
		{"batch_execution_id", ids[2], 1, ids[2]},
		{"pivot_value", ids[1], 1, ids[1]},
		{"start", "2025-02-01T00:00:00Z", 1, ids[2]},
		{"end", "2025-02-01T00:00:00Z", 1, ids[1]},
	}

	for _, tt := range tts {
		t.Run(fmt.Sprintf("API V1 testing decisions with %s filter", tt.key), func(t *testing.T) {
			ds := e.GET("/decisions").
				WithQuery(tt.key, tt.value).
				Expect().
				Status(http.StatusOK).
				JSON().Object().Path("$.data").Array()

			ds.Length().IsEqual(tt.expectedCount)

			if tt.expectedCount == 1 {
				ds.Value(0).Object().HasValue("id", tt.expectedId)
			}
		})
	}

	t.Run("API V1 testing decisions with review_status filter but no outcome", func(t *testing.T) {
		e.GET("/decisions").
			WithQuery("review_status", "pending").
			Expect().
			Status(http.StatusBadRequest)
	})

	t.Run("API V1 testing decisions with review_status filter", func(t *testing.T) {
		ds := e.GET("/decisions").
			WithQuery("outcome", "block_and_review").
			WithQuery("review_status", "pending").
			Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Array()

		ds.Length().IsEqual(1)
		ds.Value(0).Object().HasValue("id", ids[1])
	})
}
