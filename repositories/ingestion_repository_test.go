package repositories

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestCompareAndMergePayloadsWithIngestedObjects(t *testing.T) {
	testTime := time.Now()
	tests := []struct {
		name                                string
		payloads                            []models.ClientObject
		previouslyIngestedObjects           []ingestedObject
		expectedPayloadsToInsert            []models.ClientObject
		expectedObsoleteIds                 []string
		expectedValidationErrors            models.IngestionValidationErrors
		expectedExistingObjectFieldsChanged map[string][]string
	}{
		{
			name: "New payloads with no previously ingested objects",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
				},
			},
			expectedObsoleteIds:                 []string{},
			expectedValidationErrors:            models.IngestionValidationErrors{},
			expectedExistingObjectFieldsChanged: map[string][]string{},
		},
		{
			name: "Payloads with previously ingested objects",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{
				{
					id:        "1",
					objectId:  "1",
					updatedAt: testTime.Add(-time.Hour),
					data:      map[string]any{"object_id": "1", "updated_at": testTime.Add(-time.Hour)},
				},
			},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
				},
			},
			expectedObsoleteIds:      []string{"1"},
			expectedValidationErrors: models.IngestionValidationErrors{},
			expectedExistingObjectFieldsChanged: map[string][]string{
				"1": {"updated_at"},
			},
		},
		{
			name: "Payloads with missing required fields, previously ingested",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "required_field",
								Nullable: false,
							},
							ErrorIfMissing: "required_field is missing",
						},
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{
				{
					id:        "1",
					objectId:  "1",
					updatedAt: testTime.Add(-time.Hour),
					data:      map[string]any{"object_id": "1", "updated_at": testTime.Add(-time.Hour), "required_field": "present"},
				},
			},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":      "1",
						"updated_at":     testTime,
						"required_field": "present",
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "required_field",
								Nullable: false,
							},
							ErrorIfMissing: "required_field is missing",
						},
					},
				},
			},
			expectedObsoleteIds:      []string{"1"},
			expectedValidationErrors: models.IngestionValidationErrors{},
			expectedExistingObjectFieldsChanged: map[string][]string{
				"1": {"updated_at"},
			},
		},
		{
			name: "Payloads with missing required fields, not previously ingested",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "required_field",
								Nullable: false,
							},
							ErrorIfMissing: "required_field is missing",
						},
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{
				{
					id:        "1",
					objectId:  "1",
					updatedAt: testTime.Add(-time.Hour),
					data:      map[string]any{"object_id": "1", "updated_at": testTime.Add(-time.Hour)},
				},
			},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":      "1",
						"updated_at":     testTime,
						"required_field": nil,
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "required_field",
								Nullable: false,
							},
							ErrorIfMissing: "required_field is missing",
						},
					},
				},
			},
			expectedObsoleteIds: []string{"1"},
			expectedValidationErrors: models.IngestionValidationErrors{
				"1": {
					"required_field": "required_field is missing",
				},
			},
			expectedExistingObjectFieldsChanged: map[string][]string{
				"1": {"updated_at"},
			},
		},
		{
			name: "Payloads with multiple fields changed",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
						"name":       "new name",
						"age":        30,
						"email":      "new@example.com",
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{
				{
					id:        "1",
					objectId:  "1",
					updatedAt: testTime.Add(-time.Hour),
					data: map[string]any{
						"object_id": "1", "updated_at": testTime.Add(-time.Hour),
						"name": "old name", "age": 25, "email": "old@example.com",
					},
				},
			},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
						"name":       "new name",
						"age":        30,
						"email":      "new@example.com",
					},
				},
			},
			expectedObsoleteIds:      []string{"1"},
			expectedValidationErrors: models.IngestionValidationErrors{},
			expectedExistingObjectFieldsChanged: map[string][]string{
				"1": {"updated_at", "name", "age", "email"},
			},
		},
		{
			name: "Payloads with partial update (missing fields merged)",
			payloads: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
						"name":       "new name",
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "age",
								Nullable: false,
							},
						},
						{
							Field: models.Field{
								Name:     "email",
								Nullable: true,
							},
						},
					},
				},
			},
			previouslyIngestedObjects: []ingestedObject{
				{
					id:        "1",
					objectId:  "1",
					updatedAt: testTime.Add(-time.Hour),
					data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime.Add(-time.Hour),
						"name":       "old name",
						"age":        25,
						"email":      "old@example.com",
					},
				},
			},
			expectedPayloadsToInsert: []models.ClientObject{
				{
					Data: map[string]any{
						"object_id":  "1",
						"updated_at": testTime,
						"name":       "new name",
						"age":        25,
						"email":      "old@example.com",
					},
					MissingFieldsToLookup: []models.MissingField{
						{
							Field: models.Field{
								Name:     "age",
								Nullable: false,
							},
						},
						{
							Field: models.Field{
								Name:     "email",
								Nullable: true,
							},
						},
					},
				},
			},
			expectedObsoleteIds:      []string{"1"},
			expectedValidationErrors: models.IngestionValidationErrors{},
			expectedExistingObjectFieldsChanged: map[string][]string{
				"1": {"updated_at", "name"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadsToInsert, obsoleteIds, validationErrors,
				existingObjectFieldsChanged := compareAndMergePayloadsWithIngestedObjects(
				tt.payloads, tt.previouslyIngestedObjects)

			assert.Equal(t, tt.expectedPayloadsToInsert, payloadsToInsert)
			assert.Equal(t, tt.expectedObsoleteIds, obsoleteIds)
			assert.Equal(t, tt.expectedValidationErrors, validationErrors)
			assert.Equal(t, len(tt.expectedExistingObjectFieldsChanged), len(existingObjectFieldsChanged))
			for k, v := range tt.expectedExistingObjectFieldsChanged {
				assert.ElementsMatch(t, v, existingObjectFieldsChanged[k])
			}
		})
	}
}

func TestMostRecentPayloadsByObjectId(t *testing.T) {
	// Helper function to create a payload
	createPayload := func(objectId string, updatedAt time.Time) models.ClientObject {
		return models.ClientObject{
			Data: map[string]interface{}{
				"object_id":  objectId,
				"updated_at": updatedAt,
			},
		}
	}
	now := time.Now()

	t.Run("Basic Functionality", func(t *testing.T) {
		payloads := []models.ClientObject{
			createPayload("obj1", now.Add(-10*time.Hour)),
			createPayload("obj2", now.Add(-5*time.Hour)),
		}
		expectedIds := []string{"obj1", "obj2"}
		expectedPayloads := payloads

		actualIds, actualPayloads := mostRecentPayloadsByObjectId(payloads)

		assert.ElementsMatch(t, expectedIds, actualIds)
		assert.ElementsMatch(t, expectedPayloads, actualPayloads)
	})

	t.Run("Multiple Payloads for Same Object ID", func(t *testing.T) {
		payloads := []models.ClientObject{
			createPayload("obj1", now.Add(-10*time.Hour)),
			createPayload("obj1", now.Add(-5*time.Hour)), // More recent
			createPayload("obj2", now.Add(-5*time.Hour)),
		}
		expectedIds := []string{"obj1", "obj2"}
		expectedPayloads := []models.ClientObject{
			createPayload("obj1", now.Add(-5*time.Hour)),
			createPayload("obj2", now.Add(-5*time.Hour)),
		}

		actualIds, actualPayloads := mostRecentPayloadsByObjectId(payloads)

		assert.ElementsMatch(t, expectedIds, actualIds)
		assert.ElementsMatch(t, expectedPayloads, actualPayloads)
	})

	t.Run("No Payloads", func(t *testing.T) {
		payloads := []models.ClientObject{}
		expectedIds := []string{}
		expectedPayloads := []models.ClientObject{}

		actualIds, actualPayloads := mostRecentPayloadsByObjectId(payloads)

		assert.ElementsMatch(t, expectedIds, actualIds)
		assert.ElementsMatch(t, expectedPayloads, actualPayloads)
	})

	t.Run("All Payloads with Same Timestamp", func(t *testing.T) {
		timestamp := now.Add(-5 * time.Hour)
		payloads := []models.ClientObject{
			createPayload("obj1", timestamp),
			createPayload("obj1", timestamp),
			createPayload("obj2", timestamp),
		}
		expectedIds := []string{"obj1", "obj2"}
		expectedPayloads := []models.ClientObject{
			createPayload("obj1", timestamp),
			createPayload("obj2", timestamp),
		}

		actualIds, actualPayloads := mostRecentPayloadsByObjectId(payloads)

		assert.ElementsMatch(t, expectedIds, actualIds)
		assert.ElementsMatch(t, expectedPayloads, actualPayloads)
	})
}
