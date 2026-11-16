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
