package pure_utils

import (
	"github.com/Masterminds/semver"
)

// ResolveBestPromptVersion picks, among availableDirNames (candidate "vX.Y"-style names, not
// yet filtered to valid version-looking entries), the greatest one <= requestedVersion (a
// backward search: an exact match wins, otherwise the nearest earlier available version is
// used - never a newer one). Returns "" (not an error) if nothing qualifies, including when
// availableDirNames is empty - callers decide how to treat that (e.g. map it to a not-found
// error, or to "no local prompts available").
func ResolveBestPromptVersion(availableDirNames []string, requestedVersion string) (string, error) {
	reqV, err := semver.NewVersion(requestedVersion)
	if err != nil {
		return "", err
	}

	var bestVersion *semver.Version
	for _, name := range availableDirNames {
		v, err := semver.NewVersion(name)
		if err != nil {
			continue // not a version-looking name
		}
		if reqV.Compare(v) >= 0 && (bestVersion == nil || v.Compare(bestVersion) > 0) {
			bestVersion = v
		}
	}
	if bestVersion != nil {
		return bestVersion.Original(), nil
	}
	return "", nil
}
