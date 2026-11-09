package formats_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
	"github.com/checkmarble/marble-backend/usecases/sar/formats/goaml"
	"github.com/checkmarble/marble-backend/usecases/sar/formats/tracfin"
	"github.com/checkmarble/marble-backend/usecases/sar/sartest"
	"github.com/stretchr/testify/require"
)

var keys = []string{tracfin.Key, goaml.KeyLU, goaml.KeyCH}

func input(key string) sar.Input {
	return sar.Input{Pivot: sartest.Pivot(), Ext: sartest.AIExtension()[key], Settings: sartest.Settings()[key], Rates: sartest.Rates(), Now: sartest.Now(),
		Files: map[string]sar.AttachmentContent{"att1": {AttachmentID: "att1", Name: "releve-octobre.pdf", Mime: "application/pdf", Data: []byte("%PDF-1.4")}}}
}

func blocking(issues []models.SarIssue) []models.SarIssue {
	var out []models.SarIssue
	for _, i := range issues {
		if i.Severity == "error" {
			out = append(out, i)
		}
	}
	return out
}

func TestSameCaseThreeReports(t *testing.T) {
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			a, ok := sar.Get(key)
			require.True(t, ok)
			ev := sar.Evaluate(a, input(key))
			require.Empty(t, blocking(ev.Issues))
			require.Len(t, ev.Docs, 1)
			doc, err := format.XMLBytes(a.Schema(), ev.Docs[0])
			require.NoError(t, err)
			sartest.XMLLint(t, key, doc)
			view := format.Represent(a.Schema(), ev.Docs[0], ev.Issues, a.Hint)
			require.NotEmpty(t, view.Children)
		})
	}
}

func TestDumpXML(t *testing.T) {
	dir := os.Getenv("SAR_DUMP_DIR")
	if dir == "" {
		t.Skip("set SAR_DUMP_DIR to write the generated XML")
	}
	for _, key := range keys {
		a, _ := sar.Get(key)
		ev := sar.Evaluate(a, input(key))
		doc, err := format.XMLBytes(a.Schema(), ev.Docs[0])
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, strings.ReplaceAll(key, ":", "_")+".xml"), doc, 0o600))
	}
}
