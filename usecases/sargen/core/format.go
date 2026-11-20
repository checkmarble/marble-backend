package core

import "io"

// VersionStatus tells what can still be done with a format version.
type VersionStatus string

const (
	// VersionActive: creation, edition and export.
	VersionActive VersionStatus = "active"
	// VersionDeprecated: no creation; edition and export allowed, with a warning
	// that a newer version of the format exists.
	VersionDeprecated VersionStatus = "deprecated"
	// VersionRetired: no creation; edition allowed; export refused (the version
	// is no longer accepted by the authority).
	VersionRetired VersionStatus = "retired"
)

// Valid reports whether s is a known status.
func (s VersionStatus) Valid() bool {
	switch s {
	case VersionActive, VersionDeprecated, VersionRetired:
		return true
	}
	return false
}

// Format is one version of a declaration format.
type Format interface {
	Key() FormatKey
	Status() VersionStatus
	Spec() *Spec
	Validate(doc *Document) []Issue
	WriteXML(doc *Document, w io.Writer) error
}
