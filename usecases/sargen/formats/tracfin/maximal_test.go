package tracfin_test

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addItems creates `copies` items (bounded by the node maximum) in every
// repeatable block below n, and one item in every repeatable field.
func addItems(n *core.Node, parent core.Path, d *core.Document, copies int) {
	for _, c := range n.Children {
		if !c.Export || c.ExportID {
			continue
		}
		base := parent.Child(c.Name)
		if !c.IsRepeatable() {
			if c.Kind != core.NodeField {
				addItems(c, base, d, copies)
			}
			continue
		}
		count := copies
		if c.Max > 0 && c.Max < count {
			count = c.Max
		}
		if strings.HasPrefix(c.Name, "referenceFinanciere") {
			// The hand rule caps the combined references per person at 20.
			count = 1
		}
		for range count {
			item := d.AddItem(base, "k")
			if c.Kind != core.NodeField {
				addItems(c, item, d, copies)
			}
		}
	}
}

var sampleCandidates = []string{
	"Xy", "ABC1", "a.b@c.fr", "+33 1 23 45 67 89", "FR7630006000011234567890189",
	"12345", "4111111111111111", "1234567890123",
}

// sampleValue returns a valid value for the leaf n, or "" when it cannot.
func sampleValue(spec *core.Spec, n *core.Node, d *core.Document) string {
	switch n.Type {
	case core.TypeDate:
		return "2026-01-15"
	case core.TypeBoolean:
		return "false"
	case core.TypeInteger:
		return "7"
	case core.TypeDecimal:
		return "1.5"
	case core.TypeCode:
		for _, c := range spec.CodeLists[n.CodeList].Codes {
			return c.Value
		}
		return "ZZ" // unpublished list: any value (warning)
	}
	if n.Type == core.TypeRef || n.RefOrText {
		// An existing item of a target template (containers are keyed by concrete path).
		for _, t := range n.RefTargets {
			for _, container := range slices.Sorted(maps.Keys(d.Items)) {
				if keys := d.Items[container]; len(keys) > 0 && container.Template() == strings.TrimSuffix(t, "[]") {
					return string(container) + "[" + keys[0] + "]"
				}
			}
		}
		if n.Type == core.TypeRef {
			return ""
		}
	}
	for _, v := range sampleCandidates {
		if l := utf8.RuneCountInString(v); l < n.MinLength || (n.MaxLength > 0 && l > n.MaxLength) {
			continue
		}
		if n.Pattern == "" || regexp.MustCompile("^(?:"+n.Pattern+")$").MatchString(v) {
			return v
		}
	}
	return ""
}

// maximalDocument fills every exportable leaf of a document with `copies`
// items in every repeatable block.
func maximalDocument(t *testing.T, spec *core.Spec, copies int) *core.Document {
	t.Helper()
	d := core.NewDocument()
	addItems(spec.Root, "", d, copies)
	for _, p := range spec.Leaves("", d) {
		n := spec.NodeAt(p.Template())
		if n == nil || !n.Export || n.ExportID {
			continue
		}
		v := sampleValue(spec, n, d)
		require.NotEmpty(t, v, "no sample value for %s (%s %q): extend sampleCandidates", n.Template, n.Type, n.Pattern)
		d.Values[p] = core.Value{Value: v, Origin: core.OriginManual}
	}
	return d
}

// resolveRuleConflicts removes the values that a mutual-exclusion rule of the
// spec forbids in a document where every field is filled: a maximal document
// cannot satisfy exactly_one / forbidden_if rules by filling everything.
func resolveRuleConflicts(spec *core.Spec, d *core.Document, issues []core.Issue) bool {
	rules := map[string]core.Rule{}
	for _, r := range spec.Rules {
		rules[r.ID] = r
	}
	changed := false
	for _, is := range issues {
		if !is.Blocks() || is.RuleID == "" {
			continue
		}
		r := rules[is.RuleID]
		switch r.Kind {
		case "forbidden_if":
			for p := range d.Values {
				if p.HasPrefix(is.Path) {
					delete(d.Values, p)
					changed = true
				}
			}
		case "exactly_one", "at_most_one":
			// Keep the first alternative that holds a value under the scope, drop the others.
			kept := false
			for _, alt := range r.Among {
				rel := strings.TrimPrefix(strings.TrimPrefix(alt, r.Scope), ".")
				first := strings.SplitN(rel, ".", 2)[0]
				base := is.Path
				if r.Scope == "" {
					base = ""
				}
				for p := range d.Values {
					if !p.HasPrefix(base.Child(first)) {
						continue
					}
					if !kept {
						continue
					}
					delete(d.Values, p)
					changed = true
				}
				for p := range d.Values {
					if p.HasPrefix(base.Child(first)) {
						kept = true
					}
				}
			}
		}
	}
	return changed
}

func blockingOf(issues []core.Issue) []core.Issue {
	var out []core.Issue
	for _, is := range issues {
		if is.Blocks() {
			out = append(out, is)
		}
	}
	return out
}

func requireXmllint(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("xmllint"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils)")
		}
		t.Skip("xmllint not installed: XSD validation skipped")
	}
}

func xmllintOK(t *testing.T, xml []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.xml")
	require.NoError(t, os.WriteFile(path, xml, 0o600))
	out, err := exec.Command("xmllint", "--noout", "--schema", "testdata/financier-v1.4.0-test.xsd", path).CombinedOutput()
	require.NoError(t, err, "xmllint rejected the output:\n%s", string(out))
}

// TestMaximalDocument_NoBlockingIssueMeansXSDValid: one or two items in every
// repeatable block and every exportable leaf filled with a valid value. The
// validator must not block it (no false blocking on valid data) and the XML
// must be XSD-valid (no blocking issue implies a valid export).
func TestMaximalDocument_NoBlockingIssueMeansXSDValid(t *testing.T) {
	requireXmllint(t)
	spec := currentSpec(t)
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)

	for _, copies := range []int{1, 2} {
		t.Run(fmt.Sprintf("copies=%d", copies), func(t *testing.T) {
			d := maximalDocument(t, spec, copies)

			// Value-level issues mean a validator false positive or a bad sample.
			for range 10 {
				issues := f.Validate(d)
				for _, is := range blockingOf(issues) {
					require.NotEmpty(t, is.RuleID, "value issue on a valid maximal document: %s %s %s", is.Path, is.Code, is.Message)
				}
				if !resolveRuleConflicts(spec, d, issues) {
					break
				}
			}
			assert.Empty(t, blockingOf(f.Validate(d)))

			var buf bytes.Buffer
			require.NoError(t, f.WriteXML(d, &buf))
			xmllintOK(t, buf.Bytes())
		})
	}
}
