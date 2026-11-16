// Package tracfin holds the TRACFIN (France, ERMES "financier" XSD) format:
// the generated description of each supported version and its loader.
//
// Each format version lives in versions/<version>/ (pinned inputs, hand-edited
// constraints, generated spec.gen.json). The generator is gen/generate.py.
package tracfin

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// CurrentVersion is the version used for new declarations.
const CurrentVersion = "1.4.0"

const (
	familyTracfin = "tracfin"
	countryFR     = "FR"
)

//go:embed versions/*/spec.gen.json versions/*/manifest.json
var versionsFS embed.FS

type specEntry struct {
	once sync.Once
	spec *core.Spec
	ok   bool
}

var (
	specCacheMu sync.Mutex
	specCache   = map[string]*specEntry{}
)

// Key returns the format key of a TRACFIN version.
func Key(version string) core.FormatKey {
	return core.FormatKey{Family: familyTracfin, Country: countryFR, Version: version}
}

// Spec returns the generated description of a version, loaded once per version.
// The returned value is shared: callers must not modify it.
func Spec(version string) (*core.Spec, bool) {
	if _, err := fs.Stat(versionsFS, "versions/"+version+"/spec.gen.json"); err != nil {
		// Unknown version: do not grow the cache.
		return nil, false
	}
	specCacheMu.Lock()
	entry, found := specCache[version]
	if !found {
		entry = &specEntry{}
		specCache[version] = entry
	}
	specCacheMu.Unlock()

	entry.once.Do(func() {
		entry.spec, entry.ok = loadSpec(version)
	})
	return entry.spec, entry.ok
}

func loadSpec(version string) (*core.Spec, bool) {
	raw, err := versionsFS.ReadFile("versions/" + version + "/spec.gen.json")
	if err != nil {
		// Unknown version (or a name that is not a version folder).
		return nil, false
	}
	var spec core.Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		// The file is embedded and checked by tests: a parse failure is a build bug.
		panic(fmt.Sprintf("tracfin %s: spec.gen.json illisible: %v", version, err))
	}
	return &spec, true
}
