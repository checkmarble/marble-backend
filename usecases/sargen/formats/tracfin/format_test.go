package tracfin_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteXML_MinimalDocumentIsXSDValid(t *testing.T) {
	raw, err := os.ReadFile("testdata/minimal.json")
	require.NoError(t, err)
	var doc core.Document
	require.NoError(t, json.Unmarshal(raw, &doc))

	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok, "TRACFIN %s must be a known version", tracfin.CurrentVersion)

	for _, issue := range f.Validate(&doc) {
		assert.NotEqual(t, core.SeverityBlocking, issue.Severity,
			"blocking issue on minimal document: %s %s (%s)", issue.Path, issue.Code, issue.Message)
	}

	var buf bytes.Buffer
	require.NoError(t, f.WriteXML(&doc, &buf))

	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed: XSD validation skipped")
	}

	outPath := filepath.Join(t.TempDir(), "out.xml")
	require.NoError(t, os.WriteFile(outPath, buf.Bytes(), 0o600))

	cmd := exec.Command("xmllint", "--noout", "--schema", "testdata/financier-v1.4.0-test.xsd", outPath)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "xmllint rejected the output:\n%s", string(combined))
}

func TestFormat_StatusActive(t *testing.T) {
	f, ok := tracfin.New("1.4.0")
	require.True(t, ok)
	assert.Equal(t, core.VersionActive, f.Status())
}
