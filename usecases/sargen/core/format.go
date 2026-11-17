package core

import (
	"encoding/json"
	"io"
)

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

	// Plan returns the tasks to run next. The orchestrator calls it again after
	// each wave of finished tasks until it returns nothing.
	Plan(st PlanState) []TaskSpec
	// ApplyStructured applies a non-fills task output (e.g. placement): creates
	// the items and their ItemSources.
	ApplyStructured(kind string, doc *Document, out json.RawMessage) error
	// Compute derives fields from the document (backend-computed values).
	Compute(doc *Document) ComputeResult
	// Tools returns the LLM tools available for a task.
	Tools(tc ToolContext) map[string]Tool
}

// SettingsRequirer is an optional capability of a Format: the organization
// setting paths that must be filled before a generation can start, beyond what
// the spec's cardinalities already imply (e.g. an optional group the format
// nonetheless always needs). Formats that do not implement it add nothing.
type SettingsRequirer interface {
	RequiredSettings() []Path
}
