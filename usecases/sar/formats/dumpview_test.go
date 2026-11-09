package formats_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/stretchr/testify/require"
)

// TestDumpView writes a sample report view (frontend JSON language).
func TestDumpView(t *testing.T) {
	dir := os.Getenv("SAR_DUMP_DIR")
	if dir == "" {
		t.Skip("set SAR_DUMP_DIR")
	}
	a, _ := sar.Get("goaml:LU:5.0.2")
	in := input("goaml:LU:5.0.2")
	view := sar.BuildView(a, in, models.SarReport{ID: "r1", FormatKey: "goaml:LU:5.0.2", Revision: 2, PivotRevision: 3})
	raw, err := json.MarshalIndent(view, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "view-lu.json"), raw, 0o600))
}
