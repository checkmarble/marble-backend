package v1

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/h2non/gock"
)

func continuousScreenings(t *testing.T, e *httpexpect.Expect) {
	// Test validation errors
	testContinuousScreeningValidation(t, e)

	// Test 404 for non-existent config
	testContinuousScreeningNotFound(t, e)

	// Test happy path: add and delete object from monitoring
	testContinuousScreeningAddAndDelete(t, e)

	// Test skip_screen=true on existing object
	testContinuousScreeningSkipScreenTrue(t, e)
}

func testContinuousScreeningValidation(t *testing.T, e *httpexpect.Expect) {
	configStableId := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	// Test missing object_id and object_payload
	e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "account",
			ConfigStableId: configStableId,
			SkipScreen:     false,
		}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Path("$.error").Object().
		HasValue("code", "invalid_payload")

	// Test both object_id and object_payload provided
	objectId := "account-001"
	payload := json.RawMessage(`{"object_id": "account-001", "name": "John Doe"}`)

	e.POST("/continuous-screenings/objects").
		WithJSON(map[string]any{
			"object_type":      "account",
			"config_stable_id": configStableId.String(),
			"object_id":        objectId,
			"object_payload":   payload,
		}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Path("$.error").Object().
		HasValue("code", "invalid_payload")
}

func testContinuousScreeningNotFound(t *testing.T, e *httpexpect.Expect) {
	nonExistentConfigId := uuid.MustParse("99999999-9999-9999-9999-999999999999")

	// Test adding object with non-existent config
	e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "account",
			ConfigStableId: nonExistentConfigId,
			ObjectId:       utils.Ptr("account-001"),
			SkipScreen:     false,
		}).
		Expect().
		Status(http.StatusNotFound).
		JSON().Path("$.error").Object().
		HasValue("code", "not_found")

	// Test deleting object with non-existent config
	e.DELETE("/continuous-screenings/objects").
		WithJSON(params.DeleteContinuousScreeningObjectParams{
			ObjectType:     "account",
			ObjectId:       "account-001",
			ConfigStableId: nonExistentConfigId,
		}).
		Expect().
		Status(http.StatusNotFound).
		JSON().Path("$.error").Object().
		HasValue("code", "not_found")

	// Test with object type not configured in the config (config has "account", not "invalid_object_type")
	configStableId := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	// Object type not in config returns 400 Bad Request with invalid_payload
	e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "invalid_object_type",
			ConfigStableId: configStableId,
			ObjectId:       utils.Ptr("account-001"),
			SkipScreen:     false,
		}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Path("$.error").Object().
		HasValue("code", "invalid_payload")

	// Test adding object that doesn't exist in ingested data
	e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "account",
			ConfigStableId: configStableId,
			ObjectId:       utils.Ptr("non-existent-object"),
			SkipScreen:     false,
		}).
		Expect().
		Status(http.StatusNotFound).
		JSON().Path("$.error").Object().
		HasValue("code", "not_found")
}

func testContinuousScreeningAddAndDelete(t *testing.T, e *httpexpect.Expect) {
	configStableId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	objectId := "account-001"

	// Mock Yente screening endpoint
	gock.New("http://screening/match").
		Persist().
		Reply(http.StatusOK).
		BodyString(`{
			"responses": {
				"query": {
					"total": { "value": 1 },
					"results": [
						{
							"id": "sanctioned-entity-001",
							"referents": [],
							"match": true,
							"schema": "Person",
							"datasets": ["default"],
							"properties": {
								"name": ["John Doe"]
							}
						}
					]
				}
			}
		}`)
	defer gock.Off()

	// Add object to continuous screening monitoring
	resp := e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "account",
			ConfigStableId: configStableId,
			ObjectId:       &objectId,
			SkipScreen:     false,
		}).
		Expect().
		Status(http.StatusCreated).
		JSON().Path("$.data").Object()

	// Verify response contains screening result
	resp.Value("object_id").IsEqual(objectId)
	resp.Value("object_type").IsEqual("account")
	resp.Value("continuous_screening_config_stable_id").IsEqual(configStableId.String())
	resp.Value("status").String().NotEmpty()

	matches := resp.Value("matches").Array()
	matches.Length().IsEqual(1)
	matches.Value(0).Object().Value("opensanction_entity_id").IsEqual("sanctioned-entity-001")

	// Delete object from continuous screening monitoring
	e.DELETE("/continuous-screenings/objects").
		WithJSON(params.DeleteContinuousScreeningObjectParams{
			ObjectType:     "account",
			ObjectId:       objectId,
			ConfigStableId: configStableId,
		}).
		Expect().
		Status(http.StatusNoContent)

	// Verify object is no longer monitored (deleting again should return 404)
	e.DELETE("/continuous-screenings/objects").
		WithJSON(params.DeleteContinuousScreeningObjectParams{
			ObjectType:     "account",
			ObjectId:       objectId,
			ConfigStableId: configStableId,
		}).
		Expect().
		Status(http.StatusNotFound)

	// Test adding object via payload instead of object_id
	// This will update the ingested object and trigger screening (uses the same gock mock response)
	objectPayload := json.RawMessage(`{
		"object_id": "account-001",
		"updated_at": "2025-01-02T00:00:00Z",
		"name": "John Doe",
		"country": "US"
	}`)

	respPayload := e.POST("/continuous-screenings/objects").
		WithJSON(map[string]any{
			"object_type":      "account",
			"config_stable_id": configStableId.String(),
			"object_payload":   objectPayload,
			"should_screen":    true,
		}).
		Expect().
		Status(http.StatusCreated).
		JSON().Path("$.data").Object()

	respPayload.Value("object_id").IsEqual(objectId)
	respPayload.Value("object_type").IsEqual("account")

	matchesPayload := respPayload.Value("matches").Array()
	matchesPayload.Length().IsEqual(1)
	matchesPayload.Value(0).Object().Value("opensanction_entity_id").IsEqual("sanctioned-entity-001")

	// Cleanup
	e.DELETE("/continuous-screenings/objects").
		WithJSON(params.DeleteContinuousScreeningObjectParams{
			ObjectType:     "account",
			ObjectId:       objectId,
			ConfigStableId: configStableId,
		}).
		Expect().
		Status(http.StatusNoContent)
}

func testContinuousScreeningSkipScreenTrue(t *testing.T, e *httpexpect.Expect) {
	configStableId := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	objectId := "account-001"

	e.POST("/continuous-screenings/objects").
		WithJSON(params.CreateContinuousScreeningObjectParams{
			ObjectType:     "account",
			ConfigStableId: configStableId,
			ObjectId:       &objectId,
			SkipScreen:     true,
		}).
		Expect().
		Status(http.StatusNoContent).
		NoContent()

	// Cleanup
	e.DELETE("/continuous-screenings/objects").
		WithJSON(params.DeleteContinuousScreeningObjectParams{
			ObjectType:     "account",
			ObjectId:       objectId,
			ConfigStableId: configStableId,
		}).
		Expect().
		Status(http.StatusNoContent)
}
