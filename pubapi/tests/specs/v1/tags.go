package v1

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func tags(t *testing.T, e *httpexpect.Expect) {
	t.Run("GET /tags lists all tags", func(t *testing.T) {
		tags := e.GET("/tags").Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Array()

		tags.Length().IsEqual(3)

		tags.Value(0).Object().
			HasValue("id", "33333333-3333-3333-3333-333333333333").
			HasValue("name", "VIP customer").
			HasValue("target", "object")

		tags.Value(1).Object().
			HasValue("id", "22222222-2222-2222-2222-222222222222").
			HasValue("name", "High risk").
			HasValue("target", "case")

		tags.Value(2).Object().
			HasValue("id", "11111111-1111-1111-1111-111111111111").
			HasValue("name", "Fraud").
			HasValue("target", "case")
	})

	t.Run("GET /tags?target=case filters by target", func(t *testing.T) {
		tags := e.GET("/tags").WithQuery("target", "case").Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Array()

		tags.Length().IsEqual(2)
		tags.Every(func(_ int, value *httpexpect.Value) {
			value.Object().HasValue("target", "case")
		})
	})

	t.Run("GET /tags?target=object filters by target", func(t *testing.T) {
		tags := e.GET("/tags").WithQuery("target", "object").Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Array()

		tags.Length().IsEqual(1)
		tags.Value(0).Object().HasValue("name", "VIP customer")
	})

	t.Run("GET /tags pagination", func(t *testing.T) {
		resp := e.GET("/tags").WithQuery("limit", 2).Expect().
			Status(http.StatusOK).
			JSON().Object()

		resp.Path("$.data").Array().Length().IsEqual(2)
		resp.Path("$.pagination").Object().
			HasValue("has_more", true)

		nextPageId := resp.Path("$.pagination.next_page_id").String().Raw()

		page2 := e.GET("/tags").
			WithQuery("limit", 2).
			WithQuery("after", nextPageId).
			Expect().
			Status(http.StatusOK).
			JSON().Object()

		page2.Path("$.data").Array().Length().IsEqual(1)
	})

	t.Run("GET /tags?target=invalid returns 400", func(t *testing.T) {
		e.GET("/tags").WithQuery("target", "invalid").Expect().
			Status(http.StatusBadRequest)
	})

	// The first case (10000000-...) has the "Fraud" tag from fixtures.
	// The second case (11111111-...) has no tags.
	caseWithTag := "10000000-0000-0000-0000-000000000000"
	caseWithoutTag := "11111111-1111-1111-1111-111111111111"
	fraudTagId := "11111111-1111-1111-1111-111111111111"
	highRiskTagId := "22222222-2222-2222-2222-222222222222"
	objectTagId := "33333333-3333-3333-3333-333333333333"

	t.Run("POST /cases/:caseId/tags adds a tag", func(t *testing.T) {
		cas := e.POST("/cases/{caseId}/tags", caseWithoutTag).
			WithJSON(map[string]any{"tag_ids": []string{fraudTagId}}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Object()

		cas.HasValue("id", caseWithoutTag)
		caseTags := cas.Value("tags").Array()
		caseTags.Length().IsEqual(1)
		caseTags.Value(0).Object().
			HasValue("id", fraudTagId).
			HasValue("name", "Fraud")
	})

	t.Run("POST /cases/:caseId/tags is idempotent", func(t *testing.T) {
		cas := e.POST("/cases/{caseId}/tags", caseWithoutTag).
			WithJSON(map[string]any{"tag_ids": []string{fraudTagId}}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Object()

		cas.Value("tags").Array().Length().IsEqual(1)
	})

	t.Run("POST /cases/:caseId/tags adds multiple tags", func(t *testing.T) {
		cas := e.POST("/cases/{caseId}/tags", caseWithTag).
			WithJSON(map[string]any{"tag_ids": []string{highRiskTagId}}).
			Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Object()

		cas.Value("tags").Array().Length().IsEqual(2)
	})

	t.Run("POST /cases/:caseId/tags rejects object tag", func(t *testing.T) {
		e.POST("/cases/{caseId}/tags", caseWithoutTag).
			WithJSON(map[string]any{"tag_ids": []string{objectTagId}}).
			Expect().
			Status(http.StatusBadRequest)
	})

	t.Run("POST /cases/:caseId/tags rejects non-existent tag", func(t *testing.T) {
		e.POST("/cases/{caseId}/tags", caseWithoutTag).
			WithJSON(map[string]any{"tag_ids": []string{"00000000-0000-0000-0000-000000000000"}}).
			Expect().
			Status(http.StatusNotFound)
	})

	t.Run("POST /cases/:caseId/tags rejects empty body", func(t *testing.T) {
		e.POST("/cases/{caseId}/tags", caseWithoutTag).
			WithJSON(map[string]any{}).
			Expect().
			Status(http.StatusBadRequest)
	})

	t.Run("DELETE /cases/:caseId/tags/:tagId removes a tag", func(t *testing.T) {
		e.DELETE("/cases/{caseId}/tags/{tagId}", caseWithoutTag, fraudTagId).
			Expect().
			Status(http.StatusNoContent)

		// Verify it's gone
		cas := e.GET("/cases/{caseId}", caseWithoutTag).Expect().
			Status(http.StatusOK).
			JSON().Object().Path("$.data").Object()

		cas.Value("tags").Array().Length().IsEqual(0)
	})

	t.Run("DELETE /cases/:caseId/tags/:tagId is idempotent", func(t *testing.T) {
		e.DELETE("/cases/{caseId}/tags/{tagId}", caseWithoutTag, fraudTagId).
			Expect().
			Status(http.StatusNoContent)
	})

	// Clean up: remove the tag we added to caseWithTag
	t.Run("DELETE /cases/:caseId/tags/:tagId cleans up second case", func(t *testing.T) {
		e.DELETE("/cases/{caseId}/tags/{tagId}", caseWithTag, highRiskTagId).
			Expect().
			Status(http.StatusNoContent)
	})
}
