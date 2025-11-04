package utils

import (
	"net/http"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func TestOudatedVersion(t *testing.T) {
	defer gock.Off()

	releases := []GithubRelease{
		{"v0.10.0", false, time.Now().Add(-7 * 24 * time.Hour)},
		{"v0.9.2", false, time.Now().Add(-31 * 24 * time.Hour)},
		{"v0.9.1", false, time.Now().Add(-32 * 24 * time.Hour)},
		{"v0.9.0", false, time.Now().Add(-33 * 24 * time.Hour)},
		{"v0.6.0", false, time.Now().Add(-90 * 24 * time.Hour)},
		{"v0.5.0", false, time.Now().Add(-90 * 24 * time.Hour)},
		{"v0.4.0", false, time.Now().Add(-15 * 24 * time.Hour)},
		{"v0.1.0", false, time.Now().Add(-90 * 24 * time.Hour)},
	}

	gock.New("https://api.github.com").Persist().
		Get("/repos/checkmarble/marble-backend/releases").
		MatchParam("per_page", "100").
		Reply(http.StatusOK).
		JSON(releases)

	tts := []struct {
		version  string
		expected bool
	}{
		{"dev", false},                 // Development version
		{"v0.100.0", true},             // Unknown version
		{"v0.10.0", false},             // Latest version
		{"v0.10.0-10-abcd1234", false}, // Ahead of latest version
		{"v0.6.0", false},              // Old version, within minor spread tolerance
		{"v0.4.0", false},              // Old version, within grace period
		{"v0.5.0", true},               // Outdated version
		{"v0.1.0", true},               // Outdated version
	}

	for _, tt := range tts {
		t.Run(tt.version, func(t *testing.T) {
			assert.False(t, gock.HasUnmatchedRequest())
			assert.Equal(t, tt.expected, checkOutdated(t.Context(), tt.version))
		})
	}
}

func TestOudatedVersionError(t *testing.T) {
	defer gock.Off()

	gock.New("https://api.github.com").Persist().
		Get("/repos/checkmarble/marble-backend/releases").
		MatchParam("per_page", "100").
		Reply(http.StatusUnavailableForLegalReasons)

	tts := []struct {
		version  string
		expected bool
	}{
		{"dev", false},      // Development version
		{"v0.100.0", false}, // Unknown version
		{"v0.10.0", false},  // Latest version
		{"v0.6.0", false},   // Old version, within minor spread tolerance
		{"v0.4.0", false},   // Old version, within grace period
		{"v0.5.0", false},   // Outdated version
		{"v0.1.0", false},   // Outdated version
	}

	for _, tt := range tts {
		t.Run(tt.version, func(t *testing.T) {
			assert.False(t, gock.HasUnmatchedRequest())
			assert.Equal(t, tt.expected, checkOutdated(t.Context(), tt.version))
		})
	}
}
