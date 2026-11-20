package sargen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// TestDemo_BuildAndExportThroughUsecase builds the minimal valid TRACFIN
// document only through the usecase API (org settings at creation, UpdateFields
// with provisional keys, UpdateAnalysis) and exports it. The document has no
// operation line (a DS without operations is XSD-valid), so nothing is seeded
// behind the usecase's back. The reference values are those of
// formats/tracfin/testdata/minimal.json.
func TestDemo_BuildAndExportThroughUsecase(t *testing.T) {
	uc, f := newTestUsecase(t)
	f.Repo.Settings[settingsKey(f.OrgId, tracfinFamily)] = models.SarOrgFormatSettings{
		OrganizationId: f.OrgId,
		FormatFamily:   tracfinFamily,
		Values: map[string]string{
			"organisme.libelle":              "Banque Exemple SA",
			"organisme.numIdPro":             "123456789",
			"organisme.profession":           "TPI1",
			"organisme.adresse.numero":       "12",
			"organisme.adresse.nomVoie":      "Rue de la Paix",
			"organisme.adresse.codePostal":   "75002",
			"organisme.adresse.ville":        "Paris",
			"organisme.adresse.pays":         "FR",
			"organisme.adresse.tel":          "+33 1 40 00 00 00",
			"declarant.coordonnees.nom":      "Dupont",
			"declarant.coordonnees.prenom":   "Jean",
			"declarant.coordonnees.courriel": "jean.dupont@banque.fr",
			"declarant.coordonnees.tel":      "+33 1 23 45 67 89",
		},
	}
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	man := func(p, v string) core.Fill { return core.Fill{Path: core.Path(p), Value: v} }
	st, err := uc.UpdateFields(f.Ctx, id, st.Declaration.Version, []core.Fill{
		man("context.general.estPrecisionSignalement", "false"),
		man("context.general.estRequisitionJudiciaire", "false"),
		man("context.general.refInterne", "CASE-2026-0042"),
		man("context.information.correspondanceBancaire", "false"),
		man("context.information.gestionFortuneBanquePrivee", "false"),
		man("context.natureSoupcon.cadreLegal[+c]", "CL02"),
		man("context.natureSoupcon.natureSoupconPenale[+n]", "NSP07"),
		man("personnes.pp[+a].etatCivil.nomNaissance", "Martin"),
		man("personnes.pp[+a].etatCivil.prenom[+f]", "Paul"),
		man("personnes.pp[+a].implication", "Personne soupçonnée / Personne impliquée"),
		man("personnes.pp[+a].referencesFinancieres.referenceFinanciereIban[+i].iban", "FR7630006000011234567890189"),
		man("operations.dateDebut", "2026-10-01"),
		man("operations.dateFin", "2026-10-31"),
		man("operations.montantTotal", "12500.00"),
	})
	require.NoError(t, err)

	st, err = uc.UpdateAnalysis(f.Ctx, id, st.Declaration.Version,
		"Le client a recu plusieurs virements de provenance inconnue, rapidement retires en especes, sans rapport avec son activite declaree.", "")
	require.NoError(t, err)
	for _, i := range st.Issues {
		require.False(t, i.Blocks(), "blocking issue: %s %s (%s)", i.Path, i.Code, i.Message)
	}

	exp, err := uc.Export(f.Ctx, id, st.Declaration.Version)
	require.NoError(t, err)
	xml, ok := f.Blobs.Content(exp.FileReference)
	require.True(t, ok, "the export blob must be written")

	if _, err := exec.LookPath("xmllint"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils)")
		}
		t.Skip("xmllint not installed: XSD validation skipped")
	}
	outPath := filepath.Join(t.TempDir(), "out.xml")
	require.NoError(t, os.WriteFile(outPath, xml, 0o600))
	out, err := exec.Command("xmllint", "--noout", "--schema",
		"formats/tracfin/testdata/financier-v1.4.0-test.xsd", outPath).CombinedOutput()
	require.NoError(t, err, "xmllint rejected the output:\n%s", string(out))
}
