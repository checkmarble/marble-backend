package core

type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityWarning  Severity = "warning"
)

type Issue struct {
	Path     Path     `json:"path"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
	RuleID   string   `json:"rule_id,omitempty"`
}

// Blocks reports whether the issue prevents saving or exporting; a nil issue
// (value accepted) does not.
func (i *Issue) Blocks() bool {
	return i != nil && i.Severity == SeverityBlocking
}
