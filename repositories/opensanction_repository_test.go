package repositories

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestDatasetOutdatedDetector(t *testing.T) {
	type spec struct {
		schedule        string
		upstreamVersion string
		lastChange      time.Time
		localVersion    string
		updatedAt       time.Time
		expected        bool
	}

	now := time.Date(2025, 1, 23, 11, 0, 0, 0, time.UTC)

	hr := func(offset int) time.Time {
		return now.Add(time.Duration(offset) * time.Hour)
	}

	tts := []spec{
		{"", "v1", hr(0), "v1", hr(-1000), true},
		{"* * * * *", "v2", hr(0), "v1", hr(-1000), false},
		{"0 */2 * * *", "v2", hr(0), "v1", hr(-1), true},
		{"0 */2 * * *", "v2", hr(-6), "v1", hr(-7), false},
		{"0 */12 * * *", "v2", hr(-6), "v1", hr(-7), true},
		{"0 */12 * * *", "v2", hr(-6), "v1", hr(-20), false},
	}

	for _, tt := range tts {
		dataset := models.OpenSanctionsDataset{
			Version:   tt.localVersion,
			UpdatedAt: tt.updatedAt,
			Upstream: models.OpenSanctionsUpstreamDataset{
				Version:   tt.upstreamVersion,
				Schedule:  tt.schedule,
				UpdatedAt: tt.lastChange,
			},
		}

		assert.NoError(t, dataset.CheckIsUpToDate())
		assert.Equal(t, tt.expected, dataset.UpToDate)
	}
}
