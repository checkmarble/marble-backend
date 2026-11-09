package sar

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
)

// Descriptor identifies one (format, country, version) adapter.
type Descriptor struct {
	Key       string `json:"key"` // tracfin:FR:1.4.0, goaml:LU:5.0.2, goaml:CH:2025.3
	Format    string `json:"format"`
	Country   string `json:"country"`
	Version   string `json:"version"`
	Label     string `json:"label"`
	FIU       string `json:"fiu"`
	Currency  string `json:"currency"`  // reporting currency
	Packaging string `json:"packaging"` // xml_base64_attachments, zip
	Timezone  string `json:"timezone"`
}

// ExtField is one format-specific value that the pivot does not hold:
// category and indicator codes, legal grounds, report type, settings.
type ExtField struct {
	Key          string `json:"key"`
	Scope        string `json:"scope"` // report, operation, party, instrument, settings
	Label        string `json:"label"`
	Help         string `json:"help,omitempty"`
	CodeList     string `json:"code_list,omitempty"` // format schema code list
	Kind         string `json:"kind"`                // code, text, boolean, decimal, date
	MaxLength    int    `json:"max_length,omitempty"`
	Min          int    `json:"min"`
	Max          int    `json:"max"`      // -1 unbounded
	AI           bool   `json:"ai"`       // filled by the model
	Computed     bool   `json:"computed"` // set by the backend; never asked of the model
	Instructions string `json:"instructions,omitempty"`
}

// Input is everything a projection needs; it is deterministic.
type Input struct {
	Pivot    *models.SarPivot
	Ext      models.SarExtension
	Settings map[string][]string
	Rates    map[string]string
	Now      time.Time
	ReportID string
	// Files holds attachment contents at export time (keyed by attachment ID).
	Files map[string]AttachmentContent
}

func (in Input) Setting(key string) string {
	if v := in.Ext[key]; len(v.Values) > 0 {
		return v.Values[0]
	}
	if v := in.Settings[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (in Input) Values(key string) []string { return in.Ext[key].Values }
func (in Input) Value(key string) string {
	if v := in.Ext[key].Values; len(v) > 0 {
		return v[0]
	}
	return ""
}

// ExtKey builds the key of a per-object extension value.
func ExtKey(field, objectID string) string {
	if objectID == "" {
		return field
	}
	return field + "#" + objectID
}

// Attachment content for packaging.
type AttachmentContent struct {
	AttachmentID string
	Name         string
	Mime         string
	Data         []byte
}

type Artifact struct {
	Name string `json:"name"`
	Mime string `json:"mime"`
	Data []byte `json:"-"`
}

// Adapter is the contract of one (format, country, version).
type Adapter interface {
	Descriptor() Descriptor
	Schema() *format.Schema
	// Requirements are the pivot facts this format makes mandatory.
	Requirements(in Input) []models.SarIssue
	Extension() []ExtField
	// Instructions guide the model when filling AI extension fields.
	Instructions() string
	// Prepare computes backend extension values (report type, copies of
	// settings, deterministic codes). Manual values are kept.
	Prepare(in Input) models.SarExtension
	// Project builds one or more documents (goAML LU splits at 500 transactions).
	Project(in Input) ([]*format.Node, []models.SarIssue)
	Hint(normalizedPath, parentEdit string) string
	Package(docs [][]byte, attachments []AttachmentContent, in Input) (Artifact, error)
}

var registry = map[string]Adapter{}

func Register(a Adapter) { registry[a.Descriptor().Key] = a }

func Get(key string) (Adapter, bool) {
	a, ok := registry[key]
	return a, ok
}

func Adapters() []Adapter {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Adapter, 0, len(keys))
	for _, k := range keys {
		out = append(out, registry[k])
	}
	return out
}

// Evaluation is the deterministic result of pivot + extension for a format.
type Evaluation struct {
	Ext        models.SarExtension `json:"extension"`
	ReportType string              `json:"report_type"`
	Docs       []*format.Node      `json:"-"`
	Issues     []models.SarIssue   `json:"issues"`
}

func (e Evaluation) Blocking() int {
	n := 0
	for _, i := range e.Issues {
		if i.Severity == "error" {
			n++
		}
	}
	return n
}

// MergePrepared merges backend values into ext: computed fields are always
// refreshed; other fields are only filled when absent (manual/AI values win).
func MergePrepared(a Adapter, ext, prepared models.SarExtension) models.SarExtension {
	out := models.SarExtension{}
	for k, v := range ext {
		out[k] = v
	}
	computed := map[string]bool{}
	for _, f := range a.Extension() {
		computed[f.Key] = f.Computed
	}
	for k, v := range prepared {
		field := strings.SplitN(k, "#", 2)[0]
		if _, exists := out[k]; !exists || computed[field] {
			out[k] = v
		}
	}
	return out
}

// Evaluate prepares, projects and validates a report.
func Evaluate(a Adapter, in Input) Evaluation {
	in.Ext = MergePrepared(a, in.Ext, a.Prepare(in))
	ev := Evaluation{Ext: in.Ext, ReportType: in.Value("report_type")}
	for _, i := range ValidatePivot(in.Pivot) {
		if i.Severity == "error" {
			ev.Issues = append(ev.Issues, i)
		}
	}
	ev.Issues = append(ev.Issues, a.Requirements(in)...)
	ev.Issues = append(ev.Issues, ValidateExtension(a, in)...)
	docs, issues := a.Project(in)
	ev.Docs = docs
	ev.Issues = append(ev.Issues, issues...)
	for i, d := range docs {
		for _, is := range format.Validate(a.Schema(), d) {
			if len(docs) > 1 {
				is.Document = i + 1
			}
			ev.Issues = append(ev.Issues, is)
		}
	}
	ev.Issues = dedupeIssues(ev.Issues)
	return ev
}

func dedupeIssues(in []models.SarIssue) []models.SarIssue {
	seen := map[string]bool{}
	out := make([]models.SarIssue, 0, len(in))
	for _, i := range in {
		k := fmt.Sprintf("%d|%s|%s|%s", i.Document, i.Path, i.Code, i.Message)
		if !seen[k] {
			seen[k] = true
			out = append(out, i)
		}
	}
	return out
}

// ValidateExtension checks cardinality and codes of extension values.
func ValidateExtension(a Adapter, in Input) []models.SarIssue {
	var out []models.SarIssue
	key := a.Descriptor().Key
	s := a.Schema()
	fields := map[string]ExtField{}
	for _, f := range a.Extension() {
		fields[f.Key] = f
	}
	for k, v := range in.Ext {
		name := strings.SplitN(k, "#", 2)[0]
		f, ok := fields[name]
		path := "extension/" + key + "/" + k
		if !ok {
			out = append(out, models.SarIssue{Path: path, Code: "EXT_UNKNOWN_FIELD", Message: "unknown extension field " + name, Severity: "error", Layer: "extension", Edit: path})
			continue
		}
		if f.Max >= 0 && len(v.Values) > f.Max {
			out = append(out, models.SarIssue{Path: path, Code: "EXT_CARD_MAX", Message: fmt.Sprintf("%s: at most %d values", f.Label, f.Max), Severity: "error", Layer: "extension", Edit: path})
		}
		for _, val := range v.Values {
			if f.Kind == "code" && f.CodeList != "" && !s.HasCode(f.CodeList, val) {
				out = append(out, models.SarIssue{Path: path, Code: "EXT_ENUM", Message: fmt.Sprintf("%s: %q is not a code of %s", f.Label, val, f.CodeList), Severity: "error", Layer: "extension", Edit: path})
			}
			if f.MaxLength > 0 && len([]rune(val)) > f.MaxLength {
				out = append(out, models.SarIssue{Path: path, Code: "EXT_LENGTH_MAX", Message: fmt.Sprintf("%s: at most %d characters", f.Label, f.MaxLength), Severity: "error", Layer: "extension", Edit: path})
			}
		}
	}
	for _, f := range a.Extension() {
		if f.Scope != "report" && f.Scope != "settings" || f.Min == 0 {
			continue
		}
		if len(in.Ext[f.Key].Values) < f.Min && len(in.Settings[f.Key]) < f.Min {
			path := "extension/" + key + "/" + f.Key
			out = append(out, models.SarIssue{Path: path, Code: "EXT_REQUIRED", Message: f.Label + " is required", Severity: "error", Layer: "extension", Edit: path})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Meta helpers for projections.
func PivotMeta(path string, t *models.SarText) *format.Meta {
	m := &format.Meta{Edit: path}
	if t != nil {
		m.Provenance = t.P
	}
	return m
}

func ExtMeta(key, field string, v models.SarExtValue) *format.Meta {
	return &format.Meta{Edit: "extension/" + key + "/" + field, Provenance: v.P}
}

func DerivedMeta(explanation, edit string) *format.Meta {
	return &format.Meta{Derived: explanation, Edit: edit, Provenance: Calc(explanation)}
}
