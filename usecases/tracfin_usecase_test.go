package usecases

import (
	"context"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

type tracfinCases struct{ c models.Case }

func (f tracfinCases) GetCase(context.Context, string) (models.Case, error) { return f.c, nil }
func (f tracfinCases) ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error) {
	return nil, nil
}

type tracfinDM struct{ dm models.DataModel }

func (f tracfinDM) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return f.dm, nil
}

type tracfinExec struct{}

func (tracfinExec) NewExecutor() repositories.Executor { return nil }
func (tracfinExec) NewClientDbExecutor(context.Context, uuid.UUID) (repositories.Executor, error) {
	return nil, nil
}

type tracfinObjects struct {
	objects map[string]map[string]any
	calls   int
}

func (f *tracfinObjects) QueryIngestedObjectByUniqueField(_ context.Context, _ repositories.Executor, t models.Table, id, field string, _ ...string) ([]models.DataModelObject, error) {
	f.calls++
	var out []models.DataModelObject
	for _, d := range f.objects {
		if d["table"] == t.Name && d[field] == id {
			out = append(out, models.DataModelObject{Data: d})
		}
	}
	return out, nil
}
func (f *tracfinObjects) ListIngestedObjects(_ context.Context, _ repositories.Executor, t models.Table, o models.ExplorationOptions, cursor *string, limit int, _ ...string) ([]models.DataModelObject, error) {
	f.calls++
	var out []models.DataModelObject
	for _, id := range []string{"A1", "A2", "C1", "C2"} {
		d := f.objects[id]
		if d["table"] == t.Name && o.FilterFieldValue.StringValue != nil && d[o.FilterFieldName] == *o.FilterFieldValue.StringValue && (cursor == nil || id > *cursor) {
			out = append(out, models.DataModelObject{Data: d})
		}
	}
	return out[:min(limit, len(out))], nil
}
func TestTracfinNavigateSelection(t *testing.T) {
	org := uuid.New()
	objects := &tracfinObjects{objects: map[string]map[string]any{"U1": {"table": "User", "object_id": "U1"}, "A1": {"table": "Account", "object_id": "A1", "owner_id": "U1"}, "A2": {"table": "Account", "object_id": "A2", "owner_id": "U1"}, "C1": {"table": "Card", "object_id": "C1", "account_id": "A1"}, "C2": {"table": "Card", "object_id": "C2", "account_id": "A2"}}}
	dm := models.DataModel{Tables: map[string]models.Table{
		"User":    {Name: "User", SemanticType: models.SemanticTypePerson, NavigationOptions: []models.NavigationOption{{SourceFieldName: "object_id", TargetTableName: "Account", FilterFieldName: "owner_id", OrderingFieldName: "object_id", Status: models.IndexStatusValid}}},
		"Account": {Name: "Account", LinksToSingle: map[string]models.LinkToSingle{"owner": {Name: "owner", ParentTableName: "User", ParentFieldName: "object_id", ChildTableName: "Account", ChildFieldName: "owner_id"}}, NavigationOptions: []models.NavigationOption{{SourceFieldName: "object_id", TargetTableName: "Card", FilterFieldName: "account_id", OrderingFieldName: "object_id", Status: models.IndexStatusValid}}},
		"Card":    {Name: "Card", LinksToSingle: map[string]models.LinkToSingle{"account": {Name: "account", ParentTableName: "Account", ParentFieldName: "object_id", ChildTableName: "Card", ChildFieldName: "account_id"}}},
	}}
	uc := NewTracfinUsecase(org, nil, tracfinCases{models.Case{Id: "case", OrganizationId: org, Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "User", ObjectId: "U1"}}}}}, tracfinDM{dm}, objects, tracfinExec{})
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Path: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", Link: "account", Direction: "inverse"}}}}}}
	s := models.TracfinSelection{MappingID: "m", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U1"}, Bindings: []models.TracfinPathStep{{ID: "account", ParentID: "root", Table: "Account", RecordID: "A2", Link: "owner", Direction: "inverse"}, {ID: "card", ParentID: "account", Table: "Card", RecordID: "C2", Link: "account", Direction: "inverse"}}}
	ctx := context.Background()
	_, err := uc.verifySelection(ctx, "case", config, "b", s)
	require.NoError(t, err)
	s.Bindings[1].RecordID = "C1"
	_, err = uc.verifySelection(ctx, "case", config, "b", s)
	require.ErrorIs(t, err, models.BadParameterError)
	s.Bindings = s.Bindings[:1]
	page, err := uc.navigate(ctx, "case", config, s, "card", nil, 1)
	require.NoError(t, err)
	require.Len(t, page.Objects, 1)
	require.Equal(t, "C2", page.Objects[0].Data["object_id"])
	s.Bindings = nil
	page, err = uc.navigate(ctx, "case", config, s, "account", nil, 1)
	require.NoError(t, err)
	require.True(t, page.HasNextPage)
	require.Equal(t, "A1", *page.NextCursor)
	page, err = uc.navigate(ctx, "case", config, s, "account", page.NextCursor, 1)
	require.NoError(t, err)
	require.Equal(t, "A2", page.Objects[0].Data["object_id"])
	dm.Tables["User"] = models.Table{Name: "User", SemanticType: models.SemanticTypePerson}
	uc.dataModelUsecase = tracfinDM{dm}
	page, err = uc.navigate(ctx, "case", config, s, "account", nil, 1)
	require.NoError(t, err)
	require.Equal(t, "NAVIGATION_INDEX_UNAVAILABLE", page.Diagnostics[0].Code)
	uc.orgID = uuid.New()
	before := objects.calls
	_, err = uc.GetContext(ctx, "case")
	require.ErrorIs(t, err, models.NotFoundError)
	require.Equal(t, before, objects.calls)
}

type tracfinSnapshots struct {
	draft models.TracfinDraft
	calls int
}

func (f *tracfinSnapshots) GetTracfinDraft(context.Context, repositories.Executor, string, string) (models.TracfinDraft, error) {
	return f.draft, nil
}
func (f *tracfinSnapshots) GetTracfinDecisionSources(_ context.Context, _ repositories.Executor, org, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	f.calls++
	return []models.TracfinDecisionSource{{DecisionID: ids[0], HistoricalType: "Removed", Snapshot: []byte(`{"amount":10001,"currency":"EUR","object_id":9007199254740993}`)}}, nil
}
func TestTracfinDecisionWithoutIngestion(t *testing.T) {
	org := uuid.New()
	repo := &tracfinSnapshots{draft: models.TracfinDraft{OrgID: org.String(), CaseID: "case", Revision: 3, Configuration: models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", TargetField: "/ds/operations/operation/montantTotal", Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Removed", Field: "amount"}, Unit: "minor", TargetCurrency: "EUR", Currency: models.TracfinCurrencyEvidence{Field: "currency"}}}}, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "op", CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "D", HistoricalType: "Removed"}}}}}
	// nil data-model and object readers deliberately panic if snapshot resolution tries ingestion.
	uc := NewTracfinUsecase(org, repo, tracfinCases{models.Case{Id: "case", OrganizationId: org}}, nil, nil, tracfinExec{})
	got, err := uc.Prefill(context.Background(), "case")
	require.NoError(t, err)
	require.Len(t, got.Proposals, 1)
	require.Equal(t, models.TracfinDecimal("100.01"), *got.Proposals[0].Candidates[0].Decimal)
	require.Equal(t, int64(3), got.DraftRevision)
	uc.orgID = uuid.New()
	_, err = uc.Prefill(context.Background(), "case")
	require.ErrorIs(t, err, models.NotFoundError)
	require.Equal(t, 1, repo.calls)
}
func TestTracfinNavigateTwoCompanyPaths(t *testing.T) {
	org := uuid.New()
	dm := models.DataModel{Tables: map[string]models.Table{
		"User": {Name: "User", SemanticType: models.SemanticTypePerson, LinksToSingle: map[string]models.LinkToSingle{
			"employer": {Name: "employer", ChildTableName: "User", ChildFieldName: "employer_id", ParentTableName: "Company", ParentFieldName: "object_id"},
			"owner":    {Name: "owner", ChildTableName: "User", ChildFieldName: "owner_id", ParentTableName: "Company", ParentFieldName: "object_id"},
			"card":     {Name: "card", ChildTableName: "User", ChildFieldName: "card_id", ParentTableName: "Card", ParentFieldName: "object_id"},
		}}, "Company": {Name: "Company", SemanticType: models.SemanticTypeCompany}, "Card": {Name: "Card"},
	}}
	objects := &tracfinObjects{objects: map[string]map[string]any{"U": {"table": "User", "object_id": "U", "employer_id": "CO1", "owner_id": "CO2", "card_id": "C"}, "CO1": {"table": "Company", "object_id": "CO1"}, "CO2": {"table": "Company", "object_id": "CO2"}, "C": {"table": "Card", "object_id": "C"}}}
	uc := NewTracfinUsecase(org, nil, tracfinCases{models.Case{Id: "case", OrganizationId: org, Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "User", ObjectId: "U"}}}}}, tracfinDM{dm}, objects, tracfinExec{})
	config := models.TracfinConfiguration{}
	for _, name := range []string{"employer", "owner", "card"} {
		table := "Company"
		if name == "card" {
			table = "Card"
		}
		config.Mappings = append(config.Mappings, models.TracfinMapping{ID: name, Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Path: []models.TracfinPathStep{{ID: name, ParentID: "root", Table: table, Link: name, Direction: "forward"}}}})
	}
	s := models.TracfinSelection{MappingID: "employer", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}, Bindings: []models.TracfinPathStep{{ID: "employer", ParentID: "root", Table: "Company", Link: "employer", Direction: "forward", RecordID: "CO1"}, {ID: "owner", ParentID: "root", Table: "Company", Link: "owner", Direction: "forward", RecordID: "CO2"}}}
	source, err := uc.verifySelection(context.Background(), "case", config, "b", s)
	require.NoError(t, err)
	require.Equal(t, "CO1", source.Records[tracfin.SourceKey{BlockID: "b", BindingID: "employer"}].RecordID)
	require.Equal(t, "CO2", source.Records[tracfin.SourceKey{BlockID: "b", BindingID: "owner"}].RecordID)
	page, err := uc.navigate(context.Background(), "case", config, s, "card", nil, 10)
	require.NoError(t, err)
	require.Len(t, page.Objects, 1)
	require.Equal(t, "C", page.Objects[0].Data["object_id"])
	s.Bindings[1].RecordID = "CO1"
	_, err = uc.verifySelection(context.Background(), "case", config, "b", s)
	require.ErrorIs(t, err, models.BadParameterError)
}

type tracfinMissingSnapshots struct {
	tracfinSnapshots
	failure error
}

func (f *tracfinMissingSnapshots) GetTracfinDecisionSources(context.Context, repositories.Executor, string, string, []string) ([]models.TracfinDecisionSource, error) {
	return nil, f.failure
}
func TestTracfinPrefillUnavailableVersusFailure(t *testing.T) {
	org := uuid.New()
	repo := &tracfinMissingSnapshots{tracfinSnapshots: tracfinSnapshots{draft: models.TracfinDraft{OrgID: org.String(), CaseID: "case", Configuration: models.TracfinConfiguration{Mappings: []models.TracfinMapping{{ID: "m", TargetField: "/ds/operations/operation/commentaires", Source: models.TracfinMappingSource{Kind: "decision_snapshot", HistoricalType: "Removed", Field: "name"}}}}, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "op", CatalogID: "/ds/operations/operation", Selection: &models.TracfinSelection{MappingID: "m", DecisionID: "D", HistoricalType: "Removed"}}}}}}
	uc := NewTracfinUsecase(org, repo, tracfinCases{models.Case{Id: "case", OrganizationId: org}}, nil, nil, tracfinExec{})
	got, err := uc.Prefill(context.Background(), "case")
	require.NoError(t, err)
	require.Empty(t, got.Proposals)
	require.NotEmpty(t, got.Diagnostics)
	repo.failure = context.DeadlineExceeded
	_, err = uc.Prefill(context.Background(), "case")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

type tracfinPivotCases struct {
	tracfinCases
	pivots []models.PivotObject
}

func (f tracfinPivotCases) ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error) {
	return f.pivots, nil
}
func TestTracfinSelectionRootOrigins(t *testing.T) {
	org := uuid.New()
	cases := tracfinPivotCases{tracfinCases: tracfinCases{models.Case{Id: "case", OrganizationId: org, Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "User", ObjectId: "U"}}, {CaseEntityRef: models.CaseEntityRef{TableName: "Partner", ObjectId: "P"}}}}}, pivots: []models.PivotObject{{PivotType: models.PivotTypeObject, IsIngested: true, PivotObjectName: "User", PivotObjectId: "U"}, {PivotType: models.PivotTypeField, IsIngested: true, PivotObjectName: "Company", PivotObjectId: "grouping-key"}}}
	uc := NewTracfinUsecase(org, nil, cases, tracfinDM{models.DataModel{Tables: map[string]models.Table{"User": {SemanticType: models.SemanticTypePerson}, "Partner": {SemanticType: models.SemanticTypePartner}, "Company": {SemanticType: models.SemanticTypeCompany}}}}, nil, tracfinExec{})
	got, err := uc.GetContext(context.Background(), "case")
	require.NoError(t, err)
	require.Len(t, got.Roots, 1)
	require.Equal(t, []string{"case_manual_entity", "decision_pivot"}, got.Roots[0].Origins)
}

type tracfinFailingObjects struct {
	*tracfinObjects
	failure error
	queried []string
}

func (f *tracfinFailingObjects) QueryIngestedObjectByUniqueField(ctx context.Context, exec repositories.Executor, table models.Table, id, field string, extra ...string) ([]models.DataModelObject, error) {
	f.queried = append(f.queried, id)
	if id == "CO2" && f.failure != nil {
		return nil, f.failure
	}
	return f.tracfinObjects.QueryIngestedObjectByUniqueField(ctx, exec, table, id, field, extra...)
}

func TestTracfinPrefillMissingSiblingPreservesIndependentBranch(t *testing.T) {
	for _, ownerFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing first", true: "owner first"}[ownerFirst], func(t *testing.T) {
			org := uuid.New()
			employer := models.TracfinPathStep{ID: "employer", ParentID: "root", Table: "Company", Link: "employer", Direction: "forward"}
			owner := models.TracfinPathStep{ID: "owner", ParentID: "root", Table: "Company", Link: "owner", Direction: "forward"}
			child := models.TracfinPathStep{ID: "child", ParentID: "employer", Table: "Company", Link: "parent", Direction: "forward"}
			selectedEmployer, selectedOwner, selectedChild := employer, owner, child
			selectedEmployer.RecordID, selectedOwner.RecordID, selectedChild.RecordID = "CO1", "CO2", "CO3"
			bindings := []models.TracfinPathStep{selectedEmployer, selectedChild, selectedOwner}
			if ownerFirst {
				bindings = []models.TracfinPathStep{selectedOwner, selectedEmployer, selectedChild}
			}
			s := models.TracfinSelection{MappingID: "employer", Root: models.TracfinPathStep{ID: "root", Table: "User", RecordID: "U"}, Bindings: bindings}
			config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{
				{ID: "employer", TargetField: "/ds/declarant/coordonnees/nom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name", Path: []models.TracfinPathStep{employer, child}}},
				{ID: "owner", TargetField: "/ds/declarant/coordonnees/prenom", Source: models.TracfinMappingSource{Kind: "ingested", Table: "User", Field: "name", Path: []models.TracfinPathStep{owner}}},
			}}
			repo := &tracfinSnapshots{draft: models.TracfinDraft{OrgID: org.String(), CaseID: "case", Configuration: config, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "b", CatalogID: "/ds/declarant/coordonnees", Selection: &s}}}}
			dm := models.DataModel{Tables: map[string]models.Table{
				"User": {Name: "User", SemanticType: models.SemanticTypePerson, LinksToSingle: map[string]models.LinkToSingle{
					"employer": {Name: "employer", ChildTableName: "User", ChildFieldName: "employer_id", ParentTableName: "Company", ParentFieldName: "object_id"},
					"owner":    {Name: "owner", ChildTableName: "User", ChildFieldName: "owner_id", ParentTableName: "Company", ParentFieldName: "object_id"},
				}},
				"Company": {Name: "Company", SemanticType: models.SemanticTypeCompany, LinksToSingle: map[string]models.LinkToSingle{"parent": {Name: "parent", ChildTableName: "Company", ChildFieldName: "parent_id", ParentTableName: "Company", ParentFieldName: "object_id"}}},
			}}
			objects := &tracfinFailingObjects{tracfinObjects: &tracfinObjects{objects: map[string]map[string]any{"U": {"table": "User", "object_id": "U", "employer_id": "CO1", "owner_id": "CO2"}, "CO2": {"table": "Company", "object_id": "CO2", "name": "Owner"}, "CO3": {"table": "Company", "object_id": "CO3", "name": "Wrong"}}}}
			cases := tracfinCases{models.Case{Id: "case", OrganizationId: org, Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "User", ObjectId: "U"}}}}}
			uc := NewTracfinUsecase(org, repo, cases, tracfinDM{dm}, objects, tracfinExec{})
			_, err := uc.VerifySelection(context.Background(), "case", s)
			require.Error(t, err)
			objects.queried = nil
			got, err := uc.Prefill(context.Background(), "case")
			require.NoError(t, err)
			require.NotEmpty(t, got.Diagnostics)
			require.NotContains(t, objects.queried, "CO3")
			require.Len(t, got.Proposals, 1)
			require.Equal(t, "Owner", *got.Proposals[0].Candidates[0].Text)
			objects.failure = context.DeadlineExceeded
			_, err = uc.Prefill(context.Background(), "case")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			uc.orgID = uuid.New()
			_, err = uc.Prefill(context.Background(), "case")
			require.ErrorIs(t, err, models.NotFoundError)
		})
	}
}
