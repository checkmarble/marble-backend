package tracfin

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/cockroachdb/errors"
)

// rootElement is the XML root element of the TRACFIN "financier" XSD.
const rootElement = "ds"

type manifestStatus struct {
	Status core.VersionStatus `json:"status"`
}

type format struct {
	key    core.FormatKey
	status core.VersionStatus
	spec   *core.Spec
}

type formatEntry struct {
	once sync.Once
	f    *format
}

var (
	formatCacheMu sync.Mutex
	formatCache   = map[string]*formatEntry{}
)

// New returns the TRACFIN format of a known version. The instance is built
// once per version (spec and manifest parsed once) and shared: it is
// immutable, and its spec must not be modified.
func New(version string) (core.Format, bool) {
	spec, ok := Spec(version)
	if !ok {
		return nil, false
	}
	formatCacheMu.Lock()
	entry, found := formatCache[version]
	if !found {
		entry = &formatEntry{}
		formatCache[version] = entry
	}
	formatCacheMu.Unlock()

	entry.once.Do(func() {
		raw, err := versionsFS.ReadFile("versions/" + version + "/manifest.json")
		if err != nil {
			return
		}
		status, err := parseStatus(raw)
		if err != nil {
			// Embedded and checked by tests: a bad manifest is a build bug.
			panic("tracfin " + version + ": " + err.Error())
		}
		entry.f = &format{key: Key(version), status: status, spec: spec}
	})
	if entry.f == nil {
		return nil, false
	}
	return entry.f, true
}

func (f *format) Key() core.FormatKey                      { return f.key }
func (f *format) Status() core.VersionStatus               { return f.status }
func (f *format) Spec() *core.Spec                         { return f.spec }
func (f *format) Validate(doc *core.Document) []core.Issue { return Validate(f.spec, doc) }

// Plan, ApplyStructured, Compute and Tools are filled in by later tasks.
func (f *format) Plan(core.PlanState) []core.TaskSpec { return nil }
func (f *format) ApplyStructured(string, *core.Document, json.RawMessage) error {
	return nil
}
func (f *format) Compute(*core.Document) core.ComputeResult   { return core.ComputeResult{} }
func (f *format) Tools(core.ToolContext) map[string]core.Tool { return nil }

func (f *format) WriteXML(doc *core.Document, w io.Writer) error {
	return errors.Wrap(core.WriteXML(f.spec, doc, rootElement, w), "export TRACFIN")
}

// parseStatus reads the version status from a manifest.json.
func parseStatus(manifest []byte) (core.VersionStatus, error) {
	var m manifestStatus
	if err := json.Unmarshal(manifest, &m); err != nil {
		return "", errors.Wrap(err, "manifest.json illisible")
	}
	if !m.Status.Valid() {
		return "", errors.Newf("statut de version inconnu %q", m.Status)
	}
	return m.Status, nil
}
