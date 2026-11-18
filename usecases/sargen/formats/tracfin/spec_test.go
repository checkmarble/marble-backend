package tracfin_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

type manifestInput struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Encoding    string `json:"encoding"`
	SourceURL   string `json:"source_url"`
	RetrievedOn string `json:"retrieved_on"`
}

type manifest struct {
	Format string          `json:"format"`
	Status string          `json:"status"`
	Inputs []manifestInput `json:"inputs"`
}

// versionDirs lists the version folders under versions/ (relative to the package dir).
func versionDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("versions")
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	require.NotEmpty(t, out, "aucun dossier de version sous versions/")
	return out
}

// walkNodes calls visit on every descendant of n, depth first.
func walkNodes(n *core.Node, visit func(*core.Node)) {
	for _, c := range n.Children {
		visit(c)
		walkNodes(c, visit)
	}
}

func TestManifestHashesMatchInputs(t *testing.T) {
	for _, v := range versionDirs(t) {
		t.Run(v, func(t *testing.T) {
			dir := filepath.Join("versions", v)
			raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
			require.NoError(t, err)

			var m manifest
			require.NoError(t, json.Unmarshal(raw, &m))

			if v == "1.4.0" {
				// 1 XSD + 22 referentiel CSV files.
				assert.Len(t, m.Inputs, 23)
			}

			for _, in := range m.Inputs {
				content, err := os.ReadFile(filepath.Join(dir, in.Path))
				require.NoError(t, err, "entrée introuvable : %s", in.Path)

				sum := sha256.Sum256(content)
				assert.Equal(t, in.SHA256, hex.EncodeToString(sum[:]), "empreinte différente pour %s", in.Path)
			}
		})
	}
}

func TestGeneratedSpecIsUpToDate(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 absent : génération non vérifiée")
	}

	pkgDir, err := os.Getwd()
	require.NoError(t, err)

	for _, v := range versionDirs(t) {
		t.Run(v, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "spec.gen.json")

			cmd := exec.Command("python3", "-I", "gen/generate.py", "--version", v, "--out", out)
			cmd.Dir = pkgDir
			combined, err := cmd.CombinedOutput()
			require.NoError(t, err, "generate.py failed: %s", string(combined))

			generated, err := os.ReadFile(out)
			require.NoError(t, err)

			committed, err := os.ReadFile(filepath.Join(pkgDir, "versions", v, "spec.gen.json"))
			require.NoError(t, err)

			assert.True(t, bytes.Equal(committed, generated),
				"versions/%s/spec.gen.json is stale: rerun gen/generate.py", v)
		})
	}
}

func TestSpec_Shape(t *testing.T) {
	s, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)

	assert.Equal(t, "tracfin:FR:1.4.0", s.Key.String())

	pp := s.NodeAt("personnes.pp[]")
	require.NotNil(t, pp)
	assert.Equal(t, 0, pp.Min)
	assert.Equal(t, 30, pp.Max)

	analyse := s.NodeAt("analyse")
	require.NotNil(t, analyse)
	assert.Equal(t, 23000, analyse.MaxLength)

	operation := s.NodeAt("operations.operation[]")
	require.NotNil(t, operation)
	assert.Equal(t, 30, operation.Max)

	transactions := s.NodeAt("work.transactions[]")
	require.NotNil(t, transactions)
	assert.False(t, transactions.Export)

	profession := s.NodeAt("organisme.profession")
	require.NotNil(t, profession)
	prof := s.CodeLists[profession.CodeList]
	assert.NotEmpty(t, prof.Codes)

	assert.True(t, s.CodeLists["liste_ref_type_equipement.csv"].Unavailable)

	declarant := s.NodeAt("declarant")
	require.NotNil(t, declarant)
	assert.Equal(t, core.NatureSetting, declarant.Nature)

	require.NotNil(t, analyse)
	assert.Equal(t, core.NatureAnalyst, analyse.Nature)

	montantTotal := s.NodeAt("operations.montantTotal")
	require.NotNil(t, montantTotal)
	assert.Equal(t, core.NatureComputed, montantTotal.Nature)
}

func TestSpec_EveryLeafHasDecidedType(t *testing.T) {
	s, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)
	require.NotNil(t, s.Root)

	walkNodes(s.Root, func(n *core.Node) {
		if n.Kind != core.NodeField {
			return
		}
		assert.NotEmpty(t, n.Type, "champ sans type décidé : %s", n.Template)
	})

	cases := map[string]core.FieldType{
		"analyse":                 core.TypeString,
		"operations.dateDebut":    core.TypeDate,
		"operations.montantTotal": core.TypeDecimal,
	}
	for template, want := range cases {
		node := s.NodeAt(template)
		require.NotNil(t, node, "nœud introuvable : %s", template)
		assert.Equal(t, want, node.Type, template)
	}
}

func TestSpec_EveryRuleHasSource(t *testing.T) {
	s, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)
	require.NotEmpty(t, s.Rules)

	for _, rule := range s.Rules {
		assert.NotEmpty(t, rule.Source, "règle sans source : %s", rule.ID)
		assert.NotNil(t, s.NodeAt(rule.Scope), "portée de règle introuvable : %s (%q)", rule.ID, rule.Scope)
	}
}

func TestSpec_UnknownVersion(t *testing.T) {
	_, ok := tracfin.Spec("9.9.9")
	assert.False(t, ok)
}

func TestKey(t *testing.T) {
	assert.Equal(t, "tracfin:FR:1.4.0", tracfin.Key("1.4.0").String())
}

// Every list that offers "Je ne sais pas" declares it as its unknown code, and the
// code exists in the list; other lists declare none.
func TestSpec_UnknownCodes(t *testing.T) {
	s, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)

	declared := map[string]bool{}
	for name, list := range s.CodeLists {
		offersUnknown := false
		for _, c := range list.Codes {
			if c.Label == "Je ne sais pas" {
				offersUnknown = true
				assert.Equal(t, c.Value, list.UnknownCode, name)
			}
		}
		if offersUnknown {
			declared[name] = true
		} else {
			assert.Empty(t, list.UnknownCode, name)
		}
	}
	assert.Equal(t, map[string]bool{"t_choix_oui_non_nesaispas": true, "t_implication": true, "t_registre": true}, declared)
}
