package tracfin

import (
	"context"
	"os/exec"
	"testing"
)

func TestXSDValidation(t *testing.T) {
	engine, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint not installed")
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="ok" type="xs:string"/></xs:schema>`)
	for _, tc := range []struct{ xml, status string }{{`<ok>yes</ok>`, "passed"}, {`<wrong/>`, "failed"}, {`<ok>`, "failed"}, {`<!DOCTYPE ok [<!ENTITY secret SYSTEM "file:///etc/passwd">]><ok>&secret;</ok>`, "failed"}} {
		layer, diagnostics := validateXSD(context.Background(), []byte(tc.xml), schema, engine)
		if layer.Status != tc.status {
			t.Errorf("%q: got %+v, diagnostics %+v", tc.xml, layer, diagnostics)
		}
	}
}

func TestXSDUnavailable(t *testing.T) {
	layer, _ := validateXSD(context.Background(), []byte(`<ok/>`), []byte(`invalid schema`), "/nonexistent/xmllint")
	if layer.Status != "unavailable" || layer.Reason == "" {
		t.Fatalf("got %+v", layer)
	}
}

func TestOfficialXSDReportsCompilationFailure(t *testing.T) {
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint not installed")
	}
	layer, _ := ValidateOfficialXSD(context.Background(), []byte(`<ds/>`))
	if layer.Status == "passed" {
		t.Fatalf("empty declaration must not pass: %+v", layer)
	}
	if layer.Status == "unavailable" && layer.Reason == "" {
		t.Fatalf("missing reason: %+v", layer)
	}
}
