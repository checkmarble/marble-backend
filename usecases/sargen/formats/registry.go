// Package formats is the registry of the supported declaration formats.
package formats

import (
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

// Get returns the format of a known version, whatever its status.
func Get(key core.FormatKey) (core.Format, bool) {
	if key.Family == "tracfin" && key.Country == "FR" {
		return tracfin.New(key.Version)
	}
	return nil, false
}

// List returns every known format version (TRACFIN 1.4.0 only for now).
func List() []core.Format {
	var out []core.Format
	if f, ok := tracfin.New(tracfin.CurrentVersion); ok {
		out = append(out, f)
	}
	return out
}
