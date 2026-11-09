package sartest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Oracle paths of the official XSD files, relative to this package.
var Oracles = map[string]string{
	"tracfin:FR:1.4.0": "../../tracfin/profile/sources/financier-v1.4.0.xsd",
	"goaml:LU:5.0.2":   "../formats/testdata/goaml-lu-2026-10-19.xsd",
	"goaml:CH:2025.3":  "../formats/testdata/goaml-ch-2025.3.xsd",
}

func dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// XSD10 turns an official XSD into XSD 1.0 for libxml2: xs:assert are
// removed (they are re-implemented in Go and tested there) and the
// PCRE-style patterns of the TRACFIN XSD are normalized (anchors,
// non-capturing groups, unescaped dashes in classes).
func XSD10(t testing.TB, key string) string {
	raw, err := os.ReadFile(filepath.Join(dir(), Oracles[key]))
	if err != nil {
		t.Fatalf("oracle XSD: %v", err)
	}
	s := regexp.MustCompile(`(?s)<xs:assert\s.*?/>`).ReplaceAllString(string(raw), "")
	s = regexp.MustCompile(`\s(vc|xerces|altova):[a-zA-Z]+="[^"]*"`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<xs:pattern value="[^"]*"`).ReplaceAllStringFunc(s, func(m string) string {
		v := strings.TrimSuffix(strings.TrimPrefix(m, `<xs:pattern value="`), `"`)
		v = strings.TrimSuffix(strings.TrimPrefix(v, "^"), "$")
		v = strings.ReplaceAll(v, "(?:", "(")
		v = strings.ReplaceAll(v, `[\w-\._]`, `[\w\-\._]`)
		v = strings.ReplaceAll(v, `[\w-]`, `[\w\-]`)
		return `<xs:pattern value="` + v + `"`
	})
	out := filepath.Join(t.TempDir(), "schema.xsd")
	if err := os.WriteFile(out, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return out
}

// XMLLint validates doc against the XSD 1.0 oracle of a format.
func XMLLint(t testing.TB, key string, doc []byte) {
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed")
	}
	cmd := exec.Command("xmllint", "--nonet", "--noout", "--schema", XSD10(t, key), "-")
	cmd.Stdin = bytes.NewReader(doc)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("xmllint %s: %v\n%s", key, err, stderr.String())
	}
}
