package tracfin

import (
	"encoding/json"
	"io"

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

// New returns the TRACFIN format of a known version (one instance per call;
// the spec is shared and must not be modified).
func New(version string) (core.Format, bool) {
	spec, ok := Spec(version)
	if !ok {
		return nil, false
	}
	raw, err := versionsFS.ReadFile("versions/" + version + "/manifest.json")
	if err != nil {
		return nil, false
	}
	status, err := parseStatus(raw)
	if err != nil {
		// Embedded and checked by tests: a bad manifest is a build bug.
		panic("tracfin " + version + ": " + err.Error())
	}
	return &format{key: Key(version), status: status, spec: spec}, true
}

func (f *format) Key() core.FormatKey                      { return f.key }
func (f *format) Status() core.VersionStatus               { return f.status }
func (f *format) Spec() *core.Spec                         { return f.spec }
func (f *format) Validate(doc *core.Document) []core.Issue { return Validate(f.spec, doc) }

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
