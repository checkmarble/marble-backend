package tracfin

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// ValidateOfficialXSD validates XML against the embedded, unmodified financier schema.
// A missing engine, schema compilation failure or timeout is reported as unavailable.
func ValidateOfficialXSD(ctx context.Context, document []byte) (models.TracfinValidationLayer, []models.TracfinDiagnostic) {
	schema, err := profileFiles.ReadFile("profile/sources/financier-v1.4.0.xsd")
	if err != nil {
		return xsdResult("unavailable", "Embedded official XSD is unavailable")
	}
	return validateXSD(ctx, document, schema, "xmllint")
}

func xsdResult(status, reason string) (models.TracfinValidationLayer, []models.TracfinDiagnostic) {
	layer := models.TracfinValidationLayer{Status: status, Reason: reason}
	if status != "failed" {
		return layer, nil
	}
	return layer, []models.TracfinDiagnostic{{Code: "OFFICIAL_XSD_INVALID", Layer: "official_xsd", Severity: "error", Message: reason}}
}

func validateXSD(ctx context.Context, document, schema []byte, engine string) (models.TracfinValidationLayer, []models.TracfinDiagnostic) {
	// Bound input and reject DTDs before handing XML to libxml2, including local entities.
	if len(document) > 20<<20 {
		return xsdResult("unavailable", "XML exceeds the 20 MiB validation limit")
	}
	decoder := xml.NewDecoder(bytes.NewReader(document))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xsdResult("failed", "XML is not well formed")
		}
		if _, ok := token.(xml.Directive); ok {
			return xsdResult("failed", "XML directives are not allowed")
		}
	}
	dir, err := os.MkdirTemp("", "tracfin-xsd-*")
	if err != nil {
		return xsdResult("unavailable", "Cannot prepare official XSD")
	}
	defer os.RemoveAll(dir)
	schemaPath := filepath.Join(dir, "financier.xsd")
	if err := os.WriteFile(schemaPath, schema, 0600); err != nil {
		return xsdResult("unavailable", "Cannot prepare official XSD")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, engine, "--nonet", "--noout", "--schema", schemaPath, "-")
	cmd.Stdin = bytes.NewReader(document)
	// Discard tool output: it may contain XML values or local paths, and can be unbounded.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err = cmd.Run()
	if err == nil {
		return xsdResult("passed", "")
	}
	if ctx.Err() != nil {
		return xsdResult("unavailable", "Official XSD validation was cancelled or timed out")
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return xsdResult("failed", "XML does not conform to the official XSD")
	}
	return xsdResult("unavailable", "Official XSD engine could not compile or validate the schema")
}
