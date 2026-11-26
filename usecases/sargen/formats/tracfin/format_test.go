package tracfin_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils)")
		}
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

// A hyphen in the local part of an e-mail address must pass both the Go
// validator and the XSD test copy: the pattern of t_mail is
// [\w-\._]+@([\w-]+\.)+[\w-]{2,4}, and libxml2 silently drops a hyphen placed
// between \w and \. (the test copy must write it as [\w\-\._]).
func TestCourrielWithHyphenIsAcceptedAndExported(t *testing.T) {
	requireXmllint(t)
	const mail = "jean-pierre.dupont@ma-banque.fr"

	spec := currentSpec(t)
	require.Nil(t, core.ValidateValue(spec, "declarant.coordonnees.courriel", mail),
		"le validateur Go refuse une adresse valide au regard du XSD")

	raw, err := os.ReadFile("testdata/minimal.json")
	require.NoError(t, err)
	var doc core.Document
	require.NoError(t, json.Unmarshal(raw, &doc))
	setTracfinValue(&doc, "declarant.coordonnees.courriel", mail)

	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	var buf bytes.Buffer
	require.NoError(t, f.WriteXML(&doc, &buf))
	xmllintOK(t, buf.Bytes())
}

// Addresses the t_mail pattern must refuse are refused by both the Go
// validator and xmllint (the validator must not be more permissive than the XSD).
func TestCourrielInvalidIsRefusedByValidatorAndXSD(t *testing.T) {
	requireXmllint(t)
	spec := currentSpec(t)

	raw, err := os.ReadFile("testdata/minimal.json")
	require.NoError(t, err)

	for _, mail := range []string{"jean.dupont.ma-banque.fr", "jean dupont@ma-banque.fr", "jean@banque", "jean@ma-banque.fr."} {
		t.Run(mail, func(t *testing.T) {
			assert.NotNil(t, core.ValidateValue(spec, "declarant.coordonnees.courriel", mail),
				"le validateur Go accepte une adresse que le XSD refuse")

			var doc core.Document
			require.NoError(t, json.Unmarshal(raw, &doc))
			setTracfinValue(&doc, "declarant.coordonnees.courriel", mail)
			f, ok := tracfin.New(tracfin.CurrentVersion)
			require.True(t, ok)
			var buf bytes.Buffer
			require.NoError(t, f.WriteXML(&doc, &buf))

			path := filepath.Join(t.TempDir(), "out.xml")
			require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
			out, err := exec.Command("xmllint", "--noout", "--schema", "testdata/financier-v1.4.0-test.xsd", path).CombinedOutput()
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "xmllint accepted an invalid address:\n%s", string(out))
			// 3: the document does not validate (5 would be a schema that does not
			// compile); the rejection must be the e-mail pattern, in one diagnostic.
			require.Equal(t, 3, exitErr.ExitCode(), "xmllint failed for another reason:\n%s", string(out))
			assert.True(t, slices.ContainsFunc(strings.Split(string(out), "\n"), func(line string) bool {
				return strings.Contains(line, "Element 'courriel'") && strings.Contains(line, "pattern")
			}), "no courriel pattern diagnostic:\n%s", string(out))
		})
	}
}

func TestFreeTextCountry_WarnsAndIsExportedWithTypeOther(t *testing.T) {
	requireXmllint(t)
	spec := currentSpec(t)
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)

	d := maximalDocument(t, spec, 1)
	for range 10 {
		if !resolveRuleConflicts(spec, d, f.Validate(d)) {
			break
		}
	}
	require.Empty(t, blockingOf(f.Validate(d)))

	// A listed code: no issue, no attribute.
	opKeys := d.ItemKeys("operations.operation")
	require.NotEmpty(t, opKeys)
	origin := core.Path("operations.operation[" + opKeys[0] + "].paysOrigineFonds")
	setTracfinValue(d, origin, "FR")
	var listed bytes.Buffer
	require.NoError(t, f.WriteXML(d, &listed))
	assert.Contains(t, listed.String(), "<paysOrigineFonds>FR</paysOrigineFonds>")
	assert.NotContains(t, listed.String(), `type="other"`)
	for _, is := range f.Validate(d) {
		assert.NotEqual(t, origin, is.Path, "a listed code raises no issue: %s", is.Code)
	}

	// Free text: warning, not blocking, written with type="other".
	ppKeys := d.ItemKeys("personnes.pp")
	require.NotEmpty(t, ppKeys)
	nat := d.ItemKeys(core.Path("personnes.pp[" + ppKeys[0] + "].etatCivil.nationalite"))
	require.NotEmpty(t, nat)
	natPath := core.Path("personnes.pp[" + ppKeys[0] + "].etatCivil.nationalite[" + nat[0] + "]")
	setTracfinValue(d, natPath, "Kosovo du Nord")
	setTracfinValue(d, origin, "Terres australes")

	issues := f.Validate(d)
	assert.Empty(t, blockingOf(issues))
	for _, p := range []core.Path{natPath, origin} {
		is, found := findIssue(issues, core.CodeFreeTextCode, core.SeverityWarning, p)
		require.True(t, found, "free_text_code expected on %s", p)
		assert.Equal(t, p, is.Path)
		assert.Equal(t, d.Values[p].Value, is.Params["value"])
		assert.NotEmpty(t, is.Params["code_list"])
	}

	var buf bytes.Buffer
	require.NoError(t, f.WriteXML(d, &buf))
	assert.Contains(t, buf.String(), `<nationalite type="other">Kosovo du Nord</nationalite>`)
	assert.Contains(t, buf.String(), `<paysOrigineFonds type="other">Terres australes</paysOrigineFonds>`)
	xmllintOK(t, buf.Bytes())

	// Too short for the XSD (minLength 2): blocking, as before.
	setTracfinValue(d, natPath, "K")
	_, found := findIssue(f.Validate(d), core.CodeMinLength, core.SeverityBlocking, natPath)
	assert.True(t, found)
}

func TestNew_ReturnsTheSharedInstance(t *testing.T) {
	a, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	b, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	assert.Same(t, a, b)
	_, ok = tracfin.New("0.0.1")
	assert.False(t, ok)
}

// The format offers its conventions prompt to the engine, which prepends it to
// every step prompt.
func TestFormat_InstructionsPromptPath(t *testing.T) {
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	fi, ok := f.(interface{ InstructionsPromptPath() string })
	require.True(t, ok)
	assert.Equal(t, "sar_generation/tracfin/format.md", fi.InstructionsPromptPath())
}
