package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/stretchr/testify/require"
)

func settingsPrefillFixture(t *testing.T) (TracfinUsecase, *draftTestRepo, *aiTestRepo, *scriptedTracfinLLM, context.Context) {
	uc, repo, runs, llm, ctx := generationFixture(t)
	c, err := tracfin.LoadCatalog()
	require.NoError(t, err)
	settings, err := tracfinSettings([]models.TracfinSettingInput{
		{Field: models.TracfinFieldRef{FieldID: "/ds/contact/coordonnees/nom"}, Value: models.TracfinFieldInput{Kind: "text", Text: settingPtr("DUPONT")}},
		{Field: models.TracfinFieldRef{FieldID: "/ds/contact/coordonnees/tel"}, Value: models.TracfinFieldInput{Kind: "text", Text: settingPtr("+33100000000")}},
	}, c)
	require.NoError(t, err)
	repo.draft.Configuration = models.TracfinConfiguration{OrgID: repo.draft.OrgID, Settings: settings, ProfileVersion: c.Version, ProfileHash: c.Hash, Mappings: []models.TracfinMapping{
		{ID: "declarant-nom", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "settings", Field: "/ds/contact/coordonnees/nom"}},
		{ID: "declarant-tel", TargetField: "/ds/declarant/coordonnees/tel", Source: models.TracfinMappingSource{Kind: "settings", Field: "/ds/contact/coordonnees/tel"}},
	}}
	manual := "Prénom saisi"
	repo.draft.Document.Root.Children = append(repo.draft.Document.Root.Children, models.TracfinBlock{ID: "declarant", CatalogID: "/ds/declarant", Children: []models.TracfinBlock{{
		ID: "coord", CatalogID: "/ds/declarant/coordonnees", Fields: map[string][]models.TracfinValue{
			"/ds/declarant/coordonnees/prenom": {{ID: "prenom", Kind: "text", Text: &manual, Origin: &models.TracfinOrigin{Source: "manual", Resolution: "resolved"}}},
		},
	}}})
	return uc, repo, runs, llm, ctx
}

func settingPtr(v string) *string { return &v }

func declarantFields(d models.TracfinDraft) map[string][]models.TracfinValue {
	var out map[string][]models.TracfinValue
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID == "/ds/declarant/coordonnees" {
			out = b.Fields
		}
	})
	return out
}

func TestAIGenerationAppliesSettingsPrefillFirst(t *testing.T) {
	uc, repo, runs, llm, ctx := settingsPrefillFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{empty, empty, empty, empty, `{"findings":[]}`}
	var seen map[string][]models.TracfinValue
	llm.before = func(call int) {
		if call == 0 {
			seen = declarantFields(repo.draft)
		}
	}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.NotNil(t, seen, "the declarant must be prefilled before the first model call")
	require.Len(t, seen["/ds/declarant/coordonnees/nom"], 1)
	require.Len(t, seen["/ds/declarant/coordonnees/tel"], 1)
	require.Equal(t, "DUPONT", *seen["/ds/declarant/coordonnees/nom"][0].Text)
	require.Equal(t, "settings", seen["/ds/declarant/coordonnees/nom"][0].Origin.Source)
	require.Equal(t, "+33100000000", *seen["/ds/declarant/coordonnees/tel"][0].Text)
	require.Equal(t, "Prénom saisi", *seen["/ds/declarant/coordonnees/prenom"][0].Text, "manual values stay protected")
	fields := declarantFields(repo.draft)
	require.Len(t, fields["/ds/declarant/coordonnees/nom"], 1)
	require.NotEqual(t, "stale", runs.run.Status)
}

func TestAIGenerationSettingsPrefillIsIdempotentOnResume(t *testing.T) {
	uc, repo, _, llm, ctx := settingsPrefillFixture(t)
	empty := `{"blocks":[],"values":[],"attachments":[],"missing":[],"conflicts":[]}`
	llm.responses = []string{empty, empty, empty, empty, `{"findings":[]}`}
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	revision := repo.draft.Revision
	require.NoError(t, uc.GenerateAI(ctx, "case", "run"))
	require.Equal(t, revision, repo.draft.Revision)
}
