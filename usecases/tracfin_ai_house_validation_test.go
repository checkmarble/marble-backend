package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func houseReportBlock(path string, fields map[string]string, children ...models.TracfinBlock) models.TracfinBlock {
	b := models.TracfinBlock{ID: path, CatalogID: path, Fields: map[string][]models.TracfinValue{}, Children: children}
	for name, s := range fields {
		value := s
		b.Fields[path+"/"+name] = []models.TracfinValue{{ID: path + "/" + name, Kind: "text", Text: &value, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}}
	}
	return b
}
func TestAIHouseValidatedReportCanCompleteApproveAndExportWithoutOfficialXSD(t *testing.T) {
	uc, repo, runs, llm, ctx := generationFixture(t)
	c, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	root := houseReportBlock("/ds", nil,
		houseReportBlock("/ds/organisme", map[string]string{"profession": "TPI10", "libelle": "Organisation test", "numIdPro": "12345"}, houseReportBlock("/ds/organisme/adresse", map[string]string{"nomVoie": "Rue du Test", "codePostal": "75001", "ville": "Paris", "pays": "FR", "tel": "+33123456789"})),
		houseReportBlock("/ds/declarant", nil, houseReportBlock("/ds/declarant/coordonnees", map[string]string{"nom": "Martin", "prenom": "Alice", "tel": "+33123456789", "courriel": "alice@example.fr"})),
		houseReportBlock("/ds/context", nil, houseReportBlock("/ds/context/natureSoupcon", map[string]string{"cadreLegal": "CL01"}), houseReportBlock("/ds/context/general", map[string]string{"refInterne": "TEST-1", "estPrecisionSignalement": "false", "estRequisitionJudiciaire": "false"}), houseReportBlock("/ds/context/information", nil)),
		houseReportBlock("/ds/personnes", nil, houseReportBlock("/ds/personnes/pp", map[string]string{"implication": "Personne soupçonnée / Personne impliquée"}, houseReportBlock("/ds/personnes/pp/etatCivil", map[string]string{"nomNaissance": "Dupont", "prenom": "Jean"}))))
	// The ordinary editor stores booleans as booleans; reproduce that typed contract.
	general := &root.Children[2].Children[1]
	for _, name := range []string{"estPrecisionSignalement", "estRequisitionJudiciaire"} {
		value := false
		general.Fields[general.CatalogID+"/"+name] = []models.TracfinValue{{ID: name, Kind: "boolean", Boolean: &value, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}}
	}
	repo.draft.Document.Root = root
	repo.draft.Configuration = models.TracfinConfiguration{ProfileVersion: c.Version, ProfileHash: c.Hash}
	putAIAnalysis(&repo.draft, "Analyse originale <exacte> & non réécrite", "actor", "")
	runs.run.AnalysisID = repo.draft.AI.CurrentAnalysisID
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{empty, empty, empty, empty, `{"findings":[]}`}
	exports := &task6Repo{draftTestRepo: repo}
	blobs := &task6Blobs{data: map[string][]byte{}}
	uc.draftDependencies.Files = &task6Files{}
	uc.draftDependencies.Blobs = blobs
	uc.draftDependencies.Exports = exports
	uc.draftDependencies.BucketURL = "exports"
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, "completed", runs.run.Status, "validation=%+v", runs.run.Validation)
	require.Equal(t, "unavailable", runs.run.Validation.Layers["official_xsd"].Status)
	approved, err := uc.ApproveAI(ctx, "case", "run", repo.draft.Revision)
	require.NoError(t, err)
	require.Equal(t, approved.Revision, approved.AI.ReviewedRevision)
	artifact, err := uc.Export(ctx, "case", approved.Revision)
	require.NoError(t, err)
	require.Equal(t, "unavailable", artifact.Validation.Layers["official_xsd"].Status)
	require.True(t, aiValidationPassed(&artifact.Validation))
	require.Contains(t, string(blobs.data[artifact.BlobKey]), "Analyse originale &lt;exacte&gt; &amp; non réécrite")
	require.False(t, strings.Contains(string(blobs.data[artifact.BlobKey]), "<explanation>"))
}
