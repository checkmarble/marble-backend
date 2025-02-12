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
		name                      string
		payloads                  []models.ClientObject
		previouslyIngestedObjects []ingestedObject
		expectedPayloadsToInsert  []models.ClientObject
		expectedObsoleteIds       []string
		expectedValidationErrors  models.IngestionValidationErrors
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
			expectedObsoleteIds:      []string{},
			expectedValidationErrors: models.IngestionValidationErrors{},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadsToInsert, obsoleteIds, validationErrors :=
				compareAndMergePayloadsWithIngestedObjects(tt.payloads, tt.previouslyIngestedObjects)

			assert.Equal(t, tt.expectedPayloadsToInsert, payloadsToInsert)
			assert.Equal(t, tt.expectedObsoleteIds, obsoleteIds)
			assert.Equal(t, tt.expectedValidationErrors, validationErrors)
		})
	}
}
