package core

import "fmt"

// Codes of ExportError.
const (
	ExportMissingSpec = "missing_spec"
	ExportBadDate     = "bad_date"
	ExportDanglingRef = "dangling_ref"
)

// ExportError is a refusal of the XML writer: a stable code, the path concerned and parameters, with an English message.
type ExportError struct {
	Code    string
	Path    Path
	Params  map[string]string
	Message string
}

func (e *ExportError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s (at %s)", e.Message, e.Path)
}
