package v1

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func clientDataAnnotations(_ *testing.T, e *httpexpect.Expect) {
	const (
		objectType = "account"
		objectId   = "account-002"
		listPath   = "/client-data/object-type/" + objectType + "/object-id/" + objectId + "/annotations"
	)

	// List annotations: one pre-seeded comment annotation
	annotations := e.GET(listPath).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array()

	annotations.Length().IsEqual(1)

	first := annotations.Value(0).Object()
	first.
		HasValue("id", "30000000-0000-0000-0000-000000000000").
		HasValue("object_type", objectType).
		HasValue("object_id", objectId).
		HasValue("annotation_type", "comment")
	first.Value("payload").Object().HasValue("text", "This is a test comment")

	// Create a comment annotation
	created := e.POST(listPath).
		WithJSON(map[string]any{
			"type": "comment",
			"payload": map[string]string{
				"text": "A new comment",
			},
		}).
		Expect().
		Status(http.StatusCreated).
		JSON().
		Object().Path("$.data").Object()

	created.
		HasValue("object_type", objectType).
		HasValue("object_id", objectId).
		HasValue("annotation_type", "comment")
	created.Value("payload").Object().HasValue("text", "A new comment")

	// Create a risk_tag annotation
	riskTag := e.POST(listPath).
		WithJSON(map[string]any{
			"type": "risk_tag",
			"payload": map[string]string{
				"tag":    "sanctions",
				"reason": "matched on a sanctions list",
				"url":    "https://example.com/entity/123",
			},
		}).
		Expect().
		Status(http.StatusCreated).
		JSON().
		Object().Path("$.data").Object()

	riskTag.
		HasValue("object_type", objectType).
		HasValue("object_id", objectId).
		HasValue("annotation_type", "risk_tag")
	riskTag.Value("payload").Object().
		HasValue("tag", "sanctions").
		HasValue("reason", "matched on a sanctions list")

	// List contains all three annotations (1 seeded + 1 comment + 1 risk_tag)
	e.GET(listPath).
		Expect().
		Status(http.StatusOK).
		JSON().
		Object().Path("$.data").Array().
		Length().IsEqual(3)
}
