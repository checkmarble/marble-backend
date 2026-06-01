package continuous_screening

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/stretchr/testify/assert"
)

// matchesFilters (LexisNexis): a record matches when at least one enabled
// section matches; it must not slip through when no section matches.
func TestMatchesFilters_LexisNexis(t *testing.T) {
	enabled := func(datasets []string, topics map[string][]string) *models.ScreeningConfigFilter {
		return &models.ScreeningConfigFilter{Enabled: true, Datasets: datasets, Topics: topics}
	}

	// Mirrors cmd/filter.json: sanctions (with datasets) + peps + adverse_media
	// enabled, other & global disabled.
	filters := models.ScreeningConfigFilters{
		Global:       &models.ScreeningConfigFilter{Enabled: false},
		Other:        &models.ScreeningConfigFilter{Enabled: false},
		Sanctions:    enabled([]string{"ofac", "eulist"}, nil),
		Peps:         enabled(nil, nil),
		AdverseMedia: enabled(nil, nil),
	}

	job := models.EnrichedContinuousScreeningUpdateJob{
		Config: models.ContinuousScreeningConfig{
			Provider: models.ScreeningProviderLexisNexis,
			Filters:  filters,
		},
	}

	recordWithTopics := func(topics ...string) models.OpenSanctionsDeltaFileRecord {
		return models.OpenSanctionsDeltaFileRecord{
			Entity: models.OpenSanctionsDeltaFileEntity{
				Properties: map[string][]string{"topics": topics},
			},
		}
	}

	tests := []struct {
		name     string
		record   models.OpenSanctionsDeltaFileRecord
		expected bool
	}{
		{
			// Matches the enabled adverse_media section (no datasets required).
			name:     "matches an enabled section",
			record:   recordWithTopics("adverse_media"),
			expected: true,
		},
		{
			// "sanctions" topic, but no programId in the configured datasets.
			// No section matches and global must not let it through.
			name:     "matches no enabled section",
			record:   recordWithTopics("sanctions"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, matchesFilters(job, tt.record))
		})
	}
}

// Test the json decoder offset behavior and resume from offset mechanism
// Mock the S3/Blob storage with OneByteReader to simulate chunked data (stream)
func TestWork_JsonDecoderOffsetBehavior(t *testing.T) {
	record1 := `{"op": "ADD", "entity": {"id": "ent1", "schema": "Person", "datasets": ["ds1"]}}`
	record2 := `{"op": "ADD", "entity": {"id": "ent2", "schema": "Company", "datasets": ["ds2"]}}`
	record3 := `{"op": "MOD", "entity": {"id": "ent3", "schema": "Vessel", "datasets": ["ds3"]}}`

	rawContent := record1 + "\n" + record2 + "\n" + record3 + "\n"

	reader1 := iotest.OneByteReader(strings.NewReader(rawContent))
	jsonReader1 := json.NewDecoder(reader1)

	var r1 httpmodels.HTTPOpenSanctionsDeltaFileRecord
	err := jsonReader1.Decode(&r1)
	assert.NoError(t, err)

	savedOffset := jsonReader1.InputOffset()
	assert.Equal(t, int64(len(record1)), savedOffset,
		"Phase 1 offset should match exactly record1 length (before newline)")

	resumedReader := strings.NewReader(rawContent)
	_, err = resumedReader.Seek(savedOffset, io.SeekStart)
	assert.NoError(t, err)

	jsonReader2 := json.NewDecoder(iotest.OneByteReader(resumedReader))

	var r2 httpmodels.HTTPOpenSanctionsDeltaFileRecord
	err = jsonReader2.Decode(&r2)
	assert.NoError(t, err)

	// `1+` because of the newline from the previous json record
	assert.Equal(t, int64(1+len(record2)), jsonReader2.InputOffset(), "Phase 2 relative offset check")

	totalProgress := savedOffset + jsonReader2.InputOffset()
	assert.Equal(t, int64(len(record1)+1+len(record2)), totalProgress, "Phase 2 total progress check")

	var r3 httpmodels.HTTPOpenSanctionsDeltaFileRecord
	err = jsonReader2.Decode(&r3)
	assert.NoError(t, err)

	finalProgress := savedOffset + jsonReader2.InputOffset()
	assert.Equal(t, int64(len(record1)+1+len(record2)+1+len(record3)), finalProgress,
		"Phase 3 final progress should match records + intermediate newlines")
}
