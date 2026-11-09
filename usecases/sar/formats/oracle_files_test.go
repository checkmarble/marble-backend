package formats_test

import (
	"os"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sar/sartest"
)

// TestOracleFiles validates exported files against the official XSD oracles:
// SAR_ORACLE_FILES="tracfin:FR:1.4.0=/path/a.xml,goaml:LU:5.0.2=/path/b.xml"
func TestOracleFiles(t *testing.T) {
	spec := os.Getenv("SAR_ORACLE_FILES")
	if spec == "" {
		t.Skip("set SAR_ORACLE_FILES")
	}
	for _, pair := range strings.Split(spec, ",") {
		key, path, _ := strings.Cut(pair, "=")
		t.Run(key, func(t *testing.T) {
			doc, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sartest.XMLLint(t, key, doc)
		})
	}
}
