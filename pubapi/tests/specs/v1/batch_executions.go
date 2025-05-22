package v1

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func batchExecutions(t *testing.T, e *httpexpect.Expect) {
	bes := e.GET("/batch-executions").Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array()

	bes.Length().IsEqual(2)

	bes.Value(0).Object().
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("manual", true).
		HasValue("status", "processing").
		HasValue("created_at", "2025-01-01T10:00:00Z").
		Path("$.scenario").Object().
		HasValue("id", "11111111-1111-1111-1111-111111111111").
		HasValue("iteration_id", "11111111-1111-1111-1111-111111111111").
		HasValue("version", "42")

	bes.Value(1).Object().
		HasValue("id", "22222222-2222-2222-2222-222222222222").
		HasValue("manual", false).
		HasValue("status", "success").
		HasValue("decisions_created", 42).
		HasValue("created_at", "2025-01-01T08:00:00Z").
		HasValue("finished_at", "2025-01-01T09:00:00Z").
		Path("$.scenario").Object().
		HasValue("id", "22222222-2222-2222-2222-222222222222").
		HasValue("iteration_id", "22222222-2222-2222-2222-222222222222").
		HasValue("version", "42")

	e.GET("/batch-executions").
		WithQuery("scenario_id", "22222222-2222-2222-2222-222222222222").
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array().
		Length().IsEqual(1)
}
