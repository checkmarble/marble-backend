package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Regression tests for the global review of the TRACFIN branch.

func TestTracfinConfigurationRejectsInvalidMappings(t *testing.T) {
	sex := func(codes ...models.TracfinCodeMapping) models.TracfinMapping {
		m := task8Mapping("sex", t8Civil+"/sexe", "Person", "User", "gender")
		m.Codes = codes
		return m
	}
	amount := task8Snapshot("amount", t8Op+"/montantTotal", "amount")
	amount.Unit = "minor"
	for name, tc := range map[string]struct {
		mapping models.TracfinMapping
		code    string
	}{
		"unknown target code":       {sex(models.TracfinCodeMapping{Source: "F", Target: "SEX9"}), "UNKNOWN_CATALOG_CODE"},
		"contradictory client code": {sex(models.TracfinCodeMapping{Source: "F", Target: "SEX1"}, models.TracfinCodeMapping{Source: "F", Target: "SEX2"}), "CONTRADICTORY_CODE_MAPPING"},
		"amount without currency":   {amount, "MISSING_TARGET_CURRENCY"},
		"partner root":              {task8Mapping("partner", t8Civil+"/nomNaissance", "Partner", "User", "name"), "INVALID_ENTITY_TYPE"},
	} {
		t.Run(name, func(t *testing.T) {
			w := task8NewWorld(models.DataModel{}, nil, nil, nil)
			_, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{Mappings: []models.TracfinMapping{tc.mapping}})
			require.ErrorIs(t, err, models.BadParameterError)
			require.ErrorContains(t, err, tc.code)
			require.ErrorIs(t, w.repo.configErr, models.NotFoundError, "an invalid configuration is not saved")
		})
	}
	w := task8NewWorld(models.DataModel{}, nil, nil, nil)
	saved, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{Mappings: []models.TracfinMapping{sex(models.TracfinCodeMapping{Source: "F", Target: "SEX2"})}})
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.Revision)
}

// A saved selection made obsolete by a configuration or client data change is
// reported on its block; other blocks keep their proposals.
func TestTracfinPrefillStaleSelectionIsLocal(t *testing.T) {
	setup := func(t *testing.T, mappings []models.TracfinMapping) (*task8World, string, string) {
		dm, tables := task8PersonCardModel()
		w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, nil)
		w.configure(t, nil, mappings)
		d, err := w.uc.CreateDraft(w.ctx, "case", false)
		require.NoError(t, err)
		userRoot := models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}
		d = w.update(t,
			task8Add(d.Document.Root.ID, "people", "/ds/personnes"),
			task8Add("people", "pp", t8PP),
			task8Add("pp", "civil", t8Civil),
			task8Select("civil", models.TracfinSelection{MappingID: "pp-name", Root: userRoot}),
			task8Add("pp", "refs", t8PP+"/referencesFinancieres"),
			task8Add("refs", "card", t8Card),
			task8Select("card", models.TracfinSelection{MappingID: "card-pan", Root: userRoot, Bindings: []models.TracfinPathStep{task8Bound(t8Account, "A2"), task8Bound(t8CardHop, "C3")}}),
		)
		return w, task8Block(t, d, t8Civil).ID, task8Block(t, d, t8Card).ID
	}
	name := task8Mapping("pp-name", t8Civil+"/nomNaissance", "Person", "User", "last_name")
	pan := task8Mapping("card-pan", t8Card+"/@numeroCarteBancaire", "Person", "User", "pan", t8Account, t8CardHop)
	expectLocal := func(t *testing.T, w *task8World, civil, card, code string) {
		t.Helper()
		result, proposals := w.prefill(t)
		require.True(t, task8Diag(result.Diagnostics, code, card, ""), "%+v", result.Diagnostics)
		require.NotContains(t, proposals, card+" "+t8Card+"/@numeroCarteBancaire")
		p, ok := proposals[civil+" "+t8Civil+"/nomNaissance"]
		require.True(t, ok, "other blocks keep their proposals")
		d := w.update(t, task8Apply(p, 0, result.MappingVersion))
		require.Equal(t, "Martin", *task8Value(t, d, civil, t8Civil+"/nomNaissance").Text)
	}

	t.Run("source moved to another parent", func(t *testing.T) {
		w, civil, card := setup(t, []models.TracfinMapping{name, pan})
		w.objects.tables["Card"]["C3"]["account_id"] = "A1"
		expectLocal(t, w, civil, card, "SELECTION_PARENT_CHANGED")
	})
	t.Run("anchor mapping removed", func(t *testing.T) {
		w, civil, card := setup(t, []models.TracfinMapping{name, pan})
		w.configure(t, nil, []models.TracfinMapping{name})
		w.update(t, models.TracfinChangeInput{Action: "refresh_configuration"})
		expectLocal(t, w, civil, card, "MAPPING_UNAVAILABLE")
	})
	t.Run("configured path changed", func(t *testing.T) {
		w, civil, card := setup(t, []models.TracfinMapping{name, pan})
		renamed := t8CardHop
		renamed.ID = "card-v2"
		w.configure(t, nil, []models.TracfinMapping{name, task8Mapping("card-pan", t8Card+"/@numeroCarteBancaire", "Person", "User", "pan", t8Account, renamed)})
		w.update(t, models.TracfinChangeInput{Action: "refresh_configuration"})
		expectLocal(t, w, civil, card, "SELECTION_STALE")
	})
}

type task8CountingCases struct {
	task8Cases
	calls *int
}

func (f task8CountingCases) GetCase(ctx context.Context, id string) (models.Case, error) {
	*f.calls++
	return f.task8Cases.GetCase(ctx, id)
}

type task8CountingDM struct {
	tracfinDM
	calls *int
}

func (f task8CountingDM) GetDataModel(ctx context.Context, org uuid.UUID, o models.DataModelReadOptions, b bool) (models.DataModel, error) {
	*f.calls++
	return f.tracfinDM.GetDataModel(ctx, org, o, b)
}

type task8CountingRepo struct {
	*task8Repo
	decisionReads *int
}

func (r task8CountingRepo) GetTracfinDecisionSources(ctx context.Context, exec repositories.Executor, org, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	*r.decisionReads++
	return r.task8Repo.GetTracfinDecisionSources(ctx, exec, org, caseID, ids)
}

// Prefill and a batch of apply_prefill authorize the case and read each
// source once per request, independently of the number of blocks and changes.
func TestTracfinPrefillReadsSourcesOncePerRequest(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, nil)
	amount := task8Snapshot("op-amount", t8Op+"/montantTotal", "amount")
	amount.Unit, amount.TargetCurrency, amount.Currency = "minor", "EUR", models.TracfinCurrencyEvidence{Constant: "EUR"}
	w.configure(t, nil, []models.TracfinMapping{
		task8Mapping("pp-name", t8Civil+"/nomNaissance", "Person", "User", "last_name"),
		task8Mapping("pp-first", t8Civil+"/prenom", "Person", "User", "first_name"),
		amount,
	})
	d, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	changes := []models.TracfinChangeInput{
		task8Add(d.Document.Root.ID, "people", "/ds/personnes"),
		task8Add("people", "pp", t8PP),
		task8Add("pp", "civil", t8Civil),
		task8Select("civil", models.TracfinSelection{MappingID: "pp-name", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}}),
		task8Add(d.Document.Root.ID, "ops", "/ds/operations"),
	}
	for _, id := range []string{"D1", "D2", "D3", "D4"} {
		w.repo.decisions[id] = models.TracfinDecisionSource{DecisionID: id, HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"object_id":"` + id + `","amount":100}`)}
		changes = append(changes, task8Add("ops", id, t8Op), task8Select(id, models.TracfinSelection{MappingID: "op-amount", DecisionID: id, HistoricalType: "Transfer"}))
	}
	w.update(t, changes...)

	cases, dataModels, decisions := 0, 0, 0
	w.uc.cases = task8CountingCases{task8Cases: w.uc.cases.(task8Cases), calls: &cases}
	w.uc.dataModelUsecase = task8CountingDM{tracfinDM: w.uc.dataModelUsecase.(tracfinDM), calls: &dataModels}
	w.uc.repository = task8CountingRepo{task8Repo: w.repo, decisionReads: &decisions}
	reads := len(w.objects.queried)
	result, _ := w.prefill(t)
	require.Len(t, result.Proposals, 6)
	require.Equal(t, 1, cases, "case authorized once")
	require.Equal(t, 1, dataModels, "data model read once")
	require.Equal(t, 4, decisions, "one read per selected decision")
	require.Equal(t, 1, len(w.objects.queried)-reads, "one read of the selected person")

	cases, dataModels, decisions, reads = 0, 0, 0, len(w.objects.queried)
	var apply []models.TracfinChangeInput
	for _, p := range result.Proposals {
		apply = append(apply, task8Apply(p, 0, result.MappingVersion))
	}
	w.update(t, apply...)
	require.Equal(t, 1, cases, "a batch authorizes the case once")
	require.LessOrEqual(t, dataModels, 1)
	require.Equal(t, 4, decisions, "a batch reuses the sources it already read")
	require.Equal(t, 1, len(w.objects.queried)-reads)
}

// A character XML cannot represent is saved, diagnosed on its field by
// validation and refused at export with the field named, never a bare 500.
func TestTracfinForbiddenXMLCharacterIsLocated(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, nil, nil)
	_, err := w.uc.SaveConfiguration(w.ctx, w.org, models.TracfinConfigurationInput{Settings: []models.TracfinSettingInput{task8Setting("/ds/organisme/libelle", "Banque\vExemple")}})
	require.ErrorIs(t, err, models.BadParameterError, "settings are copied into every draft")
	require.ErrorContains(t, err, "/ds/organisme/libelle")

	d, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	root := d.Document.Root.ID
	d = w.update(t, task8Set(root, "/ds/analyse", task8Text("Copié\vdepuis un traitement de texte")))
	value := task8Value(t, d, root, "/ds/analyse")
	report, err := w.uc.ValidateDraft(w.ctx, "case", d.Revision)
	require.NoError(t, err)
	require.True(t, task8Diag(report.Diagnostics, "XML_FORBIDDEN_CHARACTER", root, "/ds/analyse"))
	for _, diag := range report.Diagnostics {
		if diag.Code == "XML_FORBIDDEN_CHARACTER" {
			require.Equal(t, value.ID, diag.Field.ValueID)
		}
	}
	_, err = w.uc.Export(w.ctx, "case", d.Revision)
	require.ErrorIs(t, err, models.UnprocessableEntityError)
	require.ErrorContains(t, err, "/ds/analyse")
	require.ErrorContains(t, err, "U+000B")
	require.Nil(t, w.repo.draft.LastExport)
	require.Empty(t, w.blobs.data, "no artifact is published")
}

func TestTracfinPeriodProposalAppliedAndExported(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, nil, nil)
	start := task8Snapshot("start", "/ds/operations/dateDebut", "executed_at")
	end := task8Snapshot("end", "/ds/operations/dateFin", "executed_at")
	w.configure(t, nil, []models.TracfinMapping{start, end})
	w.repo.decisions["D1"] = models.TracfinDecisionSource{DecisionID: "D1", HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"executed_at":"2026-09-30T23:30:00Z"}`)}
	w.repo.decisions["D2"] = models.TracfinDecisionSource{DecisionID: "D2", HistoricalType: "Transfer", Snapshot: json.RawMessage(`{"executed_at":"2026-08-01T08:00:00Z"}`)}
	d, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	d = w.update(t,
		task8Add(d.Document.Root.ID, "ops", "/ds/operations"),
		task8Add("ops", "op1", t8Op), task8Select("op1", models.TracfinSelection{MappingID: "start", DecisionID: "D1", HistoricalType: "Transfer"}),
		task8Add("ops", "op2", t8Op), task8Select("op2", models.TracfinSelection{MappingID: "start", DecisionID: "D2", HistoricalType: "Transfer"}),
	)
	ops := task8Block(t, d, "/ds/operations").ID
	result, proposals := w.prefill(t)
	require.False(t, task8Diag(result.Diagnostics, "PARTIAL_PERIOD", "", ""), "every selected line has a date")
	var apply []models.TracfinChangeInput
	for _, field := range []string{"/ds/operations/dateDebut", "/ds/operations/dateFin"} {
		apply = append(apply, task8Apply(proposals[ops+" "+field], 0, result.MappingVersion))
	}
	d = w.update(t, apply...)
	require.Equal(t, "calculated", task8Value(t, d, ops, "/ds/operations/dateDebut").Origin.Source)
	_, _, x := w.export(t)
	require.Equal(t, []string{"01/08/2026"}, x.texts("operations/dateDebut"))
	require.Equal(t, []string{"30/09/2026"}, x.texts("operations/dateFin"))
}

type task8DeniedCases struct{ task8Cases }

func (task8DeniedCases) GetCase(context.Context, string) (models.Case, error) {
	return models.Case{}, models.ForbiddenError
}

// Losing case access refuses every TRACFIN capability without side effects.
func TestTracfinCaseAccessDeniedEverywhere(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, nil)
	d, err := w.uc.CreateDraft(w.ctx, "case", false)
	require.NoError(t, err)
	d = w.update(t, task8Set(d.Document.Root.ID, "/ds/analyse", task8Text("Analyse")))
	e, err := w.uc.Export(w.ctx, "case", d.Revision)
	require.NoError(t, err)
	before := w.repo.draft
	artifacts := len(w.blobs.data)

	w.uc.cases = task8DeniedCases{w.uc.cases.(task8Cases)}
	w.repo.exists = false
	_, err = w.uc.CreateDraft(w.ctx, "case", false)
	require.ErrorIs(t, err, models.ForbiddenError)
	w.repo.exists = true
	for name, call := range map[string]func() error{
		"context":  func() error { _, err := w.uc.GetContext(w.ctx, "case"); return err },
		"draft":    func() error { _, err := w.uc.GetDraft(w.ctx, "case"); return err },
		"update":   func() error { _, err := w.uc.UpdateDraft(w.ctx, "case", models.TracfinDraftInput{ExpectedRevision: d.Revision, Changes: []models.TracfinChangeInput{task8Set(d.Document.Root.ID, "/ds/analyse", task8Text("x"))}}); return err },
		"navigate": func() error { _, err := w.uc.Navigate(w.ctx, "case", models.TracfinSelection{}, "account", nil, 10); return err },
		"prefill":  func() error { _, err := w.uc.PrefillAtRevision(w.ctx, "case", d.Revision); return err },
		"validate": func() error { _, err := w.uc.ValidateDraft(w.ctx, "case", d.Revision); return err },
		"export":   func() error { _, err := w.uc.Export(w.ctx, "case", d.Revision); return err },
		"download": func() error { _, _, err := w.uc.DownloadExport(w.ctx, "case", e.ID); return err },
	} {
		require.ErrorIs(t, call(), models.ForbiddenError, name)
	}
	require.Equal(t, before, w.repo.draft)
	require.Len(t, w.blobs.data, artifacts)
}
