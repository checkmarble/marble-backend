package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func prefillEditFixture(t *testing.T) (TracfinUsecase, *draftTestRepo, *tracfinObjects, uuid.UUID) {
	t.Helper()
	org := uuid.New()
	config := models.TracfinConfiguration{Revision: 1, Mappings: []models.TracfinMapping{
		{ID: "name", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name"}},
		{ID: "first", TargetField: "/ds/declarant/coordonnees/prenom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "first"}},
	}}
	selection := models.TracfinSelection{MappingID: "name", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}}
	repo := &draftTestRepo{config: config, exists: true, draft: models.TracfinDraft{OrgID: org.String(), CaseID: "case", Revision: 4, Configuration: config, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "p", CatalogID: "/ds/declarant/coordonnees", Selection: &selection}}}}
	objects := &tracfinObjects{objects: map[string]map[string]any{"U": {"table": "User", "object_id": "U", "name": "Source", "first": "First"}}}
	cases := tracfinCases{models.Case{Id: "case", OrganizationId: org, Outcome: models.CaseConfirmedRisk, Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "User", ObjectId: "U"}}}}}
	dm := tracfinDM{models.DataModel{Tables: map[string]models.Table{"User": {Name: "User", SemanticType: models.SemanticTypePerson}}}}
	uc := NewTracfinUsecase(org, repo, cases, dm, objects, tracfinExec{}, TracfinDraftDependencies{Repository: repo, Transactions: draftTestTransactions{}})
	return uc, repo, objects, org
}
func prefillCommand(p tracfin.PrefillProposal, version string) models.TracfinChangeInput {
	return models.TracfinChangeInput{Action: "apply_prefill", ProposalID: p.ID, CandidateID: p.Candidates[0].ID, MappingVersion: version}
}
func TestTracfinDraftPrefillBatchAndUntrustedCandidates(t *testing.T) {
	uc, repo, objects, org := prefillEditFixture(t)
	ctx := draftTestContext(org)
	result, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Len(t, result.Proposals, 2)
	var nameProposal tracfin.PrefillProposal
	for _, p := range result.Proposals {
		if p.Field.FieldID == "/ds/declarant/coordonnees/nom" {
			nameProposal = p
		}
	}
	require.NotEmpty(t, nameProposal.ID)
	good := prefillCommand(nameProposal, result.MappingVersion)
	for name, invalid := range map[string]models.TracfinChangeInput{
		"forged candidate": func() models.TracfinChangeInput { c := good; c.CandidateID = "forged"; return c }(),
		"stale mapping":    func() models.TracfinChangeInput { c := good; c.MappingVersion = "stale"; return c }(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: 4, Changes: []models.TracfinChangeInput{invalid}})
			require.Error(t, err)
			require.Equal(t, int64(4), repo.draft.Revision)
			require.Empty(t, repo.draft.Document.Root.Fields)
		})
	}
	// A proposal from an earlier source read cannot authorize a changed source value.
	objects.objects["U"]["name"] = "changed"
	_, err = uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: 4, Changes: []models.TracfinChangeInput{good}})
	require.Error(t, err)
	require.Equal(t, int64(4), repo.draft.Revision)
	objects.objects["U"]["name"] = "Source"
	result, err = uc.Prefill(ctx, "case")
	require.NoError(t, err)
	commands := make([]models.TracfinChangeInput, 0, len(result.Proposals))
	for _, p := range result.Proposals {
		commands = append(commands, prefillCommand(p, result.MappingVersion))
	}
	got, err := uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: result.DraftRevision, Changes: commands})
	require.NoError(t, err)
	require.Equal(t, "Source", *findDraftField(got.Document.Root, "/ds/declarant/coordonnees/nom")[0].Text)
	require.Equal(t, "First", *findDraftField(got.Document.Root, "/ds/declarant/coordonnees/prenom")[0].Text)
	for _, p := range result.Proposals {
		v := findDraftField(got.Document.Root, p.Field.FieldID)[0]
		require.NotEmpty(t, v.ID)
		require.NotEqual(t, p.Candidates[0].ID, v.ID)
		require.Equal(t, "ingested", v.Origin.Source)
		require.NotEmpty(t, v.Origin.Sources)
	}
	_, err = uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: 4, Changes: commands})
	require.ErrorIs(t, err, models.ConflictError)
}

func TestTracfinDraftRefreshKeepsCorrectionsAndRejectsOldMapping(t *testing.T) {
	uc, repo, objects, org := prefillEditFixture(t)
	ctx := draftTestContext(org)
	proposal, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Len(t, proposal.Proposals, 2)
	input := func(s string) *models.TracfinFieldInput { return &models.TracfinFieldInput{Kind: "text", Text: &s} }
	edited, err := uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: 4, Changes: []models.TracfinChangeInput{
		{Action: "set_field", Field: models.TracfinFieldRef{BlockID: "p", FieldID: "/ds/declarant/coordonnees/nom"}, Value: input("Manual")},
		{Action: "clear_field", Field: models.TracfinFieldRef{BlockID: "p", FieldID: "/ds/declarant/coordonnees/prenom"}},
	}})
	require.NoError(t, err)
	name := findDraftField(edited.Document.Root, "/ds/declarant/coordonnees/nom")[0]
	clear := findDraftField(edited.Document.Root, "/ds/declarant/coordonnees/prenom")[0]
	repo.config = edited.Configuration
	repo.config.Revision++
	repo.config.Mappings[0].Source.Field = "renamed"
	objects.objects["U"]["renamed"] = "New source"
	// A global update alone does not change the copied configuration.
	require.Equal(t, edited.Configuration.Revision, repo.draft.Configuration.Revision)
	refreshed, err := uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: edited.Revision, Changes: []models.TracfinChangeInput{{Action: "refresh_configuration"}}})
	require.NoError(t, err)
	require.Equal(t, repo.config.Revision, refreshed.Configuration.Revision)
	gotName := findDraftField(refreshed.Document.Root, "/ds/declarant/coordonnees/nom")[0]
	gotClear := findDraftField(refreshed.Document.Root, "/ds/declarant/coordonnees/prenom")[0]
	require.Equal(t, name.ID, gotName.ID)
	require.Equal(t, "Manual", *gotName.Text)
	require.Equal(t, clear.ID, gotClear.ID)
	require.Equal(t, "cleared", gotClear.Kind)
	_, err = uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: refreshed.Revision, Changes: []models.TracfinChangeInput{prefillCommand(proposal.Proposals[0], proposal.MappingVersion)}})
	require.Error(t, err)
	require.Equal(t, refreshed.Revision, repo.draft.Revision)
	result, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Empty(t, result.Proposals)
}

func TestTracfinDraftConflictChoiceRequiresNewSourceDecision(t *testing.T) {
	uc, repo, objects, org := prefillEditFixture(t)
	ctx := draftTestContext(org)
	repo.draft.Configuration.Mappings[1].TargetField = "/ds/declarant/coordonnees/nom"
	before, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Len(t, before.Proposals, 1)
	require.Len(t, before.Proposals[0].Candidates, 2)
	require.Equal(t, "conflict", before.Proposals[0].Resolution)
	chosen := prefillCommand(before.Proposals[0], before.MappingVersion)
	saved, err := uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: 4, Changes: []models.TracfinChangeInput{chosen}})
	require.NoError(t, err)
	selected := findDraftField(saved.Document.Root, "/ds/declarant/coordonnees/nom")[0]
	require.NotNil(t, selected.Text)
	require.False(t, selected.NeedsReview)
	stable, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Equal(t, saved.Revision, repo.draft.Revision)
	require.Equal(t, *selected.Text, *findDraftField(repo.draft.Document.Root, "/ds/declarant/coordonnees/nom")[0].Text)
	require.False(t, findDraftField(repo.draft.Document.Root, "/ds/declarant/coordonnees/nom")[0].NeedsReview)
	require.NotEmpty(t, stable.MappingVersion)
	objects.objects["U"]["name"] = "new source value"
	fresh, err := uc.Prefill(ctx, "case")
	require.NoError(t, err)
	require.Len(t, fresh.Proposals, 1)
	values := []string{}
	for _, candidate := range fresh.Proposals[0].Candidates {
		if candidate.Text != nil {
			values = append(values, *candidate.Text)
		}
	}
	require.Contains(t, values, "new source value")
	require.Equal(t, saved.Revision, repo.draft.Revision)
	require.Equal(t, *selected.Text, *findDraftField(repo.draft.Document.Root, "/ds/declarant/coordonnees/nom")[0].Text)
	_, err = uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: saved.Revision, Changes: []models.TracfinChangeInput{chosen}})
	require.Error(t, err)
	require.Equal(t, saved.Revision, repo.draft.Revision)
}
