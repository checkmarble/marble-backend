package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Masterminds/semver"
)

type GithubRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HtmlUrl     string    `json:"html_url"`
	Body        string    `json:"body"`
}

type OutdatedInfo struct {
	Outdated      bool     `json:"outdated"`
	LatestVersion string   `json:"latest_version,omitempty"`
	LatestUrl     string   `json:"latest_url,omitempty"`
	ReleaseNotes  []string `json:"release_notes,omitempty"`
}

const (
	GithubBackendReleaseUrl     = "https://api.github.com/repos/checkmarble/marble-backend/releases?per_page=100"
	GithubReleaseUrl            = "https://api.github.com/repos/checkmarble/marble/releases?per_page=100"
	GithubReleaseMaxMinorSpread = 2
	GithubReleaseGracePeriod    = 30 * 24 * time.Hour
)

var (
	Outdated      OutdatedInfo
	OutdatedMutex sync.RWMutex
)

func RunCheckOutdated(appVersion string) {
	for {
		info := checkOutdated(context.Background(), appVersion)

		OutdatedMutex.Lock()
		Outdated = info
		OutdatedMutex.Unlock()

		time.Sleep(time.Hour)
	}
}

func checkOutdated(ctx context.Context, appVersion string) (info OutdatedInfo) {
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

	releases, err := fetchReleases(ctx, GithubBackendReleaseUrl)
	// In case we cannot retrieve releases or the list is empty, bail out.
	if err != nil {
		return
	}
	if len(releases) == 0 {
		return
	}

	var (
		currentRelease *GithubRelease
		minorsSince    = 1
	)

	latest := releases[0]
	latestSv, err := semver.NewVersion(latest.TagName)
	if err != nil {
		return
	}

	// Keep the latest unique minor version found, so we can count them.
	var seenMinor *semver.Version

	info.ReleaseNotes = make([]string, 0, minorsSince)

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

	if releases, err := fetchReleases(ctx, GithubReleaseUrl); err == nil && len(releases) > 0 {
		release := releases[0]
		info.LatestVersion = release.TagName
		info.LatestUrl = release.HtmlUrl

		for _, release := range releases {
			sv, err := semver.NewVersion(release.TagName)
			if err != nil {
				continue
			}

			if sv.Major() == currentSv.Major() && sv.Minor() == currentSv.Minor() {
				break
			}

			info.ReleaseNotes = append(info.ReleaseNotes, release.Body)
		}
	}

	// Edge case, if we did not find the current version in the release list, if
	// means we are outdated by more than 100 releases or we are in a weird state.
	if currentRelease == nil {
		info.Outdated = true
		return
	}

	// Two conditions must be true for the version be considered obsolete:
	//  * Be at least two minors away from the latest
	//  * Be at least a month old
	if minorsSince > GithubReleaseMaxMinorSpread &&
		latestSv.GreaterThan(currentSv) &&
		latest.PublishedAt.Sub(currentRelease.PublishedAt) > GithubReleaseGracePeriod {

		info.Outdated = true
	}

	return
}

func fetchReleases(ctx context.Context, url string) ([]GithubRelease, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	var releases []GithubRelease

	if err := dec.Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}
