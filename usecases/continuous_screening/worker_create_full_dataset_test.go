package continuous_screening

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeFTMPropertyValue(t *testing.T) {
	tests := []struct {
		name     string
		property models.FollowTheMoneyProperty
		value    string
		expected string
	}{
		// Country normalization
		{
			name:     "Country - USA to us",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "USA",
			expected: "us",
		},
		{
			name:     "Nationality - France to fr",
			property: models.FollowTheMoneyPropertyNationality,
			value:    "France",
			expected: "fr",
		},
		// Date normalization
		{
			name:     "Unchanged property",
			property: models.FollowTheMoneyPropertyName,
			value:    "John Doe",
			expected: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFTMPropertyValue(tt.property, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeCountryFTMPropertyValue(t *testing.T) {
	tests := []struct {
		name     string
		property models.FollowTheMoneyProperty
		value    string
		expected string
	}{
		// Country property tests
		{
			name:     "Country - Alpha-2 code",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "US",
			expected: "us",
		},
		{
			name:     "Country - Alpha-3 code",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "USA",
			expected: "us",
		},
		{
			name:     "Country - Full name",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "France",
			expected: "fr",
		},
		{
			name:     "Country - Typo",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "Germeny",
			expected: "de",
		},
		{
			name:     "Country - ISO 3166-2 subdivision",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "CH-AI",
			expected: "ch",
		},
		{
			name:     "Country - Invalid returns original value",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "invalid123",
			expected: "invalid123",
		},

		// Nationality property tests
		{
			name:     "Nationality - Full name",
			property: models.FollowTheMoneyPropertyNationality,
			value:    "Germany",
			expected: "de",
		},
		{
			name:     "Nationality - Alpha-2",
			property: models.FollowTheMoneyPropertyNationality,
			value:    "FR",
			expected: "fr",
		},

		// BirthCountry property tests
		{
			name:     "BirthCountry - Full name",
			property: models.FollowTheMoneyPropertyBirthCountry,
			value:    "Brazil",
			expected: "br",
		},

		// Citizenship property tests
		{
			name:     "Citizenship - Full name",
			property: models.FollowTheMoneyPropertyCitizenship,
			value:    "Japan",
			expected: "jp",
		},

		// Jurisdiction property tests
		{
			name:     "Jurisdiction - Full name",
			property: models.FollowTheMoneyPropertyJurisdiction,
			value:    "United Kingdom",
			expected: "gb",
		},

		// Non-country properties should pass through unchanged
		{
			name:     "Name property - unchanged",
			property: models.FollowTheMoneyPropertyName,
			value:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "Email property - unchanged",
			property: models.FollowTheMoneyPropertyEmail,
			value:    "test@example.com",
			expected: "test@example.com",
		},
		{
			name:     "Address property - unchanged",
			property: models.FollowTheMoneyPropertyAddress,
			value:    "123 Main St",
			expected: "123 Main St",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeCountryFTMPropertyValue(tt.property, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildDatasetEntity_CountryNormalization(t *testing.T) {
	ftmEntity := models.FollowTheMoneyEntityPerson
	ftmPropertyCountry := models.FollowTheMoneyPropertyCountry
	ftmPropertyName := models.FollowTheMoneyPropertyName

	table := models.Table{
		Name:      "customers",
		FTMEntity: &ftmEntity,
		Fields: map[string]models.Field{
			"name": {
				Name:        "name",
				FTMProperty: &ftmPropertyName,
			},
			"country": {
				Name:        "country",
				FTMProperty: &ftmPropertyCountry,
			},
		},
	}

	track := models.ContinuousScreeningDeltaTrack{
		EntityId: "test-entity-id",
	}

	tests := []struct {
		name            string
		ingestedData    map[string]any
		expectedCountry []string
	}{
		{
			name: "Full country name normalized to lowercase alpha-2",
			ingestedData: map[string]any{
				"name":    "John Doe",
				"country": "United States",
			},
			expectedCountry: []string{"us"},
		},
		{
			name: "Alpha-3 code normalized to lowercase alpha-2",
			ingestedData: map[string]any{
				"name":    "Jane Doe",
				"country": "GBR",
			},
			expectedCountry: []string{"gb"},
		},
		{
			name: "ISO 3166-2 subdivision normalized to parent country",
			ingestedData: map[string]any{
				"name":    "Hans Mueller",
				"country": "CH-ZH",
			},
			expectedCountry: []string{"ch"},
		},
		{
			name: "Invalid country value is kept as-is",
			ingestedData: map[string]any{
				"name":    "Invalid Person",
				"country": "invalid123xyz",
			},
			expectedCountry: []string{"invalid123xyz"}, // invalid values are kept unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingestedObject := models.DataModelObject{
				Data: tt.ingestedData,
			}

			result, err := buildDatasetEntity(table, track, ingestedObject)
			assert.NoError(t, err)

			countryVal, ok := result.Properties["country"]
			assert.True(t, ok, "country property should exist")
			assert.Equal(t, tt.expectedCountry, countryVal)

			// Check metadata in notes
			notesVal, ok := result.Properties["notes"]
			assert.True(t, ok, "notes property should exist")
			assert.Len(t, notesVal, 1)

			var metadata models.EntityNoteMetadata
			err = json.Unmarshal([]byte(notesVal[0]), &metadata)
			assert.NoError(t, err)
			assert.Equal(t, track.ObjectId, metadata.ObjectId)
			assert.Equal(t, track.ObjectType, metadata.ObjectType)
		})
	}
}

func TestBuildDatasetEntity_Metadata(t *testing.T) {
	ftmEntity := models.FollowTheMoneyEntityPerson
	ftmPropertyName := models.FollowTheMoneyPropertyName

	table := models.Table{
		Name:      "customers",
		FTMEntity: &ftmEntity,
		Fields: map[string]models.Field{
			"name": {
				Name:        "name",
				FTMProperty: &ftmPropertyName,
			},
		},
	}

	track := models.ContinuousScreeningDeltaTrack{
		EntityId:   "test-entity-id",
		ObjectId:   "obj-123",
		ObjectType: "customer",
	}

	ingestedObject := models.DataModelObject{
		Data: map[string]any{
			"name": "John Doe",
		},
	}

	result, err := buildDatasetEntity(table, track, ingestedObject)
	assert.NoError(t, err)

	notesVal, ok := result.Properties["notes"]
	assert.True(t, ok, "notes property should exist")
	assert.Len(t, notesVal, 1)

	var metadata models.EntityNoteMetadata
	err = json.Unmarshal([]byte(notesVal[0]), &metadata)
	assert.NoError(t, err)
	assert.Equal(t, "obj-123", metadata.ObjectId)
	assert.Equal(t, "customer", metadata.ObjectType)
}

func TestGenerateNextVersion(t *testing.T) {
	now := time.Date(2026, 1, 6, 14, 45, 3, 0, time.UTC)
	expectedPrefix := "20260106144503"

	tests := []struct {
		name            string
		previousVersion string
		expected        string
	}{
		{
			name:            "Empty previous version",
			previousVersion: "",
			expected:        expectedPrefix + "-001",
		},
		{
			name:            "Previous version from different time",
			previousVersion: "20260106144502-001",
			expected:        expectedPrefix + "-001",
		},
		{
			name:            "Previous version from same time",
			previousVersion: "20260106144503-001",
			expected:        expectedPrefix + "-002",
		},
		{
			name:            "Previous version from same time, higher index",
			previousVersion: "20260106144503-099",
			expected:        expectedPrefix + "-100",
		},
		{
			name:            "Clock rollback - current time earlier than previous",
			previousVersion: "20260106144504-001",
			expected:        "20260106144504-002", // Should increment previous to stay string incrementable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNextVersion(tt.previousVersion, now)
			assert.Equal(t, tt.expected, result)
			if tt.previousVersion != "" {
				assert.True(t, result > tt.previousVersion,
					"New version %s should be string incrementable (> %s)", result, tt.previousVersion)
			}
		})
	}
}

func TestNormalizeDateFTMPropertyValue(t *testing.T) {
	tests := []struct {
		name     string
		property models.FollowTheMoneyProperty
		value    string
		expected string
	}{
		// BirthDate property tests - various formats
		{
			name:     "BirthDate - ISO 8601 format",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "1990-05-15",
			expected: "1990-05-15",
		},
		{
			name:     "BirthDate - Slash format YYYY/MM/DD",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "1990/05/15",
			expected: "1990-05-15",
		},
		{
			name:     "BirthDate - ISO 8601 with time",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "1990-05-15T10:30:00",
			expected: "1990-05-15",
		},
		{
			name:     "BirthDate - RFC3339",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "1990-05-15T10:30:00Z",
			expected: "1990-05-15",
		},
		{
			name:     "BirthDate - Compact YYYYMMDD",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "19900515",
			expected: "1990-05-15",
		},
		{
			name:     "BirthDate - Invalid format returns original",
			property: models.FollowTheMoneyPropertyBirthDate,
			value:    "not-a-date",
			expected: "not-a-date",
		},

		// DeathDate property tests
		{
			name:     "DeathDate - ISO 8601 format",
			property: models.FollowTheMoneyPropertyDeathDate,
			value:    "2020-12-31",
			expected: "2020-12-31",
		},

		// Non-date properties should pass through unchanged
		{
			name:     "Name property - unchanged",
			property: models.FollowTheMoneyPropertyName,
			value:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "Country property - unchanged (date normalization should not affect)",
			property: models.FollowTheMoneyPropertyCountry,
			value:    "France",
			expected: "France",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDateFTMPropertyValue(tt.property, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildDatasetEntity_DateNormalization(t *testing.T) {
	ftmEntity := models.FollowTheMoneyEntityPerson
	ftmPropertyName := models.FollowTheMoneyPropertyName
	ftmPropertyBirthDate := models.FollowTheMoneyPropertyBirthDate

	table := models.Table{
		Name:      "customers",
		FTMEntity: &ftmEntity,
		Fields: map[string]models.Field{
			"name": {
				Name:        "name",
				FTMProperty: &ftmPropertyName,
			},
			"birth_date": {
				Name:        "birth_date",
				FTMProperty: &ftmPropertyBirthDate,
			},
		},
	}

	track := models.ContinuousScreeningDeltaTrack{
		EntityId: "test-entity-id",
	}

	tests := []struct {
		name              string
		ingestedData      map[string]any
		expectedBirthDate []string
	}{
		{
			name: "ISO date format normalized",
			ingestedData: map[string]any{
				"name":       "John Doe",
				"birth_date": "1990-05-15",
			},
			expectedBirthDate: []string{"1990-05-15"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingestedObject := models.DataModelObject{
				Data: tt.ingestedData,
			}

			result, err := buildDatasetEntity(table, track, ingestedObject)
			assert.NoError(t, err)

			birthDateVal, ok := result.Properties["birthDate"]
			assert.True(t, ok, "birthDate property should exist")
			assert.Equal(t, tt.expectedBirthDate, birthDateVal)
		})
	}
}
