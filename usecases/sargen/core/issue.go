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
	// Params carries the values a front end needs to translate the issue from
	// its code; numbers are decimal strings. Nil when the code has none.
	Params map[string]string `json:"params,omitempty"`
}

// Params builds an issue parameter map from key/value pairs; it returns nil
// when there are none. A trailing key without a value is ignored.
func Params(kv ...string) map[string]string {
	if len(kv) < 2 {
		return nil
	}
	m := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// Blocks reports whether the issue prevents saving or exporting; a nil issue
// (value accepted) does not.
func (i *Issue) Blocks() bool {
	return i != nil && i.Severity == SeverityBlocking
}
