package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver"
)

type GithubRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

const (
	GithubReleaseUrl            = "https://api.github.com/repos/checkmarble/marble-backend/releases?per_page=100"
	GithubReleaseMaxMinorSpread = 2
	GithubReleaseGracePeriod    = 30 * 24 * time.Hour
)

var IsOutdatedVersion atomic.Bool

func RunCheckOutdated(appVersion string) {
	for {
		IsOutdatedVersion.Store(checkOutdated(context.Background(), appVersion))
		time.Sleep(time.Hour)
	}
}

func checkOutdated(ctx context.Context, appVersion string) (isOutdated bool) {
	// No outdated notion for development builds
	if appVersion == "dev" {
		return
	}

	currentSv, err := semver.NewVersion(appVersion)
	if err != nil {
		return
	}
	withoutPrerelease, _ := currentSv.SetPrerelease("")
	currentSv = &withoutPrerelease

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, GithubReleaseUrl, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	var (
		releases       []GithubRelease
		currentRelease *GithubRelease
		minorsSince    = 1
	)

	// In case we cannot retrieve releases or the list is empty, bail out.
	if err := dec.Decode(&releases); err != nil {
		return
	}
	if len(releases) == 0 {
		return
	}

	latest := releases[0]
	latestSv, err := semver.NewVersion(latest.TagName)
	if err != nil {
		return
	}

	// Keep the latest unique minor version found, so we can count them.
	var seenMinor *semver.Version

	for _, r := range releases {
		if r.TagName == currentSv.Original() {
			currentRelease = &r
			break
		}

		// Let's only count real releases
		if !r.Prerelease {
			if sv, err := semver.NewVersion(r.TagName); err == nil {
				// If this is a different minor version from the last one seen, let's bump the minor counter.
				// Here, [1.2.0, 1.2.1, 1.3.1] counts for two versions (spread of `1`).
				if seenMinor != nil && (sv.Major() != seenMinor.Major() || sv.Minor() != seenMinor.Minor()) {
					minorsSince += 1
				}

				seenMinor = sv
			}
		}
	}

	// Edge case, if we did not find the current version in the release list, if
	// means we are outdated by more than 100 releases or we are in a weird state.
	if currentRelease == nil {
		isOutdated = true
		return
	}

	// Two conditions must be true for the version be considered obsolete:
	//  * Be at least two minors away from the latest
	//  * Be at least a month old
	if minorsSince > GithubReleaseMaxMinorSpread &&
		latestSv.GreaterThan(currentSv) &&
		latest.PublishedAt.Sub(currentRelease.PublishedAt) > GithubReleaseGracePeriod {

		isOutdated = true
	}

	return
}
