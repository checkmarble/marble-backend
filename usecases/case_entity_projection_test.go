package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCaseEntitiesMerge(t *testing.T) {
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	manual := []models.CaseManualEntity{{CaseEntityRef: ref}, {CaseEntityRef: models.CaseEntityRef{TableName: "old", ObjectId: "gone"}}}
	got := mergeCaseEntities(manual, []models.CaseEntityRef{ref, ref}, map[models.CaseEntityRef]models.DataModelObject{ref: {Data: map[string]any{"name": "Alice"}}})
	require.Len(t, got, 2)
	require.Equal(t, ref, got[0].CaseEntityRef)
	require.Equal(t, []models.CaseEntitySource{models.CaseEntitySourceManual, models.CaseEntitySourceDecision}, got[0].Sources)
	require.Equal(t, "Alice", got[0].Data["name"])
	require.Nil(t, got[1].Data)
	remaining := mergeCaseEntities(nil, []models.CaseEntityRef{ref}, nil)
	require.Equal(t, []models.CaseEntitySource{models.CaseEntitySourceDecision}, remaining[0].Sources)
	require.NotNil(t, mergeCaseEntities(nil, nil, nil))
}

type caseEntityPivotsStub struct {
	ingestedDataReaderRepository
	pivots []models.PivotMetadata
	err    error
}

func (s caseEntityPivotsStub) ListPivots(context.Context, repositories.Executor, uuid.UUID, *string, bool, bool) ([]models.PivotMetadata, error) {
	return s.pivots, s.err
}

type caseEntityResolutionObjectsStub struct {
	caseEntityObjectsStub
	err error
}

func (s *caseEntityResolutionObjectsStub) QueryIngestedObjectByUniqueField(_ context.Context, _ repositories.Executor, _ models.Table, value, field string, _ ...string) ([]models.DataModelObject, error) {
	if value == "alice@example.com" && field == "email" {
		return s.objects, s.err
	}
	return nil, s.err
}
func (s *caseEntityResolutionObjectsStub) QueryIngestedObjectsByIds(context.Context, repositories.Executor, models.Table, []string) ([]models.DataModelObject, error) {
	return s.objects, s.err
}
func (caseEntityExecutorStub) NewExecutor() repositories.Executor { return nil }

func TestCaseEntitiesResolveCanonicalDecisionIdentity(t *testing.T) {
	objectIdField, emailField, fieldField := "object-field", "email-field", "field-field"
	pivots := []models.PivotMetadata{{Id: uuid.New(), BaseTableId: "customers-table", FieldId: &objectIdField}, {Id: uuid.New(), BaseTableId: "customers-table", FieldId: &emailField}, {Id: uuid.New(), BaseTableId: "customers-table", FieldId: &fieldField}, {Id: uuid.New(), BaseTableId: "customers-table"}}
	dm := models.DataModel{Tables: map[string]models.Table{"customers": {ID: "customers-table", Name: "customers", Fields: map[string]models.Field{
		"object_id": {ID: objectIdField, Name: "object_id", UnicityConstraint: models.ActiveUniqueConstraint},
		"email":     {ID: emailField, Name: "email", UnicityConstraint: models.ActiveUniqueConstraint},
		"city":      {ID: fieldField, Name: "city"},
	}}}}
	objects := &caseEntityResolutionObjectsStub{caseEntityObjectsStub: caseEntityObjectsStub{objects: []models.DataModelObject{{Data: map[string]any{"object_id": "c-123"}}}}}
	uc := IngestedDataReaderUsecase{repository: caseEntityPivotsStub{pivots: pivots}, dataModelUsecase: caseEntityModelStub{model: dm}, clientDbRepository: objects, executorFactory: caseEntityExecutorStub{}}
	values := []models.PivotDataWithCount{{PivotId: pivots[0].Id.String(), PivotValue: "known"}, {PivotId: pivots[1].Id.String(), PivotValue: "alice@example.com"}, {PivotId: pivots[2].Id.String(), PivotValue: "Paris"}, {PivotId: pivots[3].Id.String(), PivotValue: "unresolved"}}
	got, err := uc.ReadCaseDecisionEntityRefs(context.Background(), uuid.New(), values)
	require.NoError(t, err)
	require.ElementsMatch(t, []models.CaseEntityRef{{TableName: "customers", ObjectId: "known"}, {TableName: "customers", ObjectId: "c-123"}}, got)
	objects.err = errors.New("client database down")
	got, err = uc.ReadCaseDecisionEntityRefs(context.Background(), uuid.New(), values)
	require.Error(t, err)
	require.True(t, isCaseEntityClientDBError(err))
	require.Equal(t, []models.CaseEntityRef{{TableName: "customers", ObjectId: "known"}}, got)
	_, err = uc.ReadCaseEntityObjects(context.Background(), uuid.New(), got)
	require.True(t, isCaseEntityClientDBError(err))
	uc.repository = caseEntityPivotsStub{err: errors.New("Marble down")}
	_, err = uc.ReadCaseDecisionEntityRefs(context.Background(), uuid.New(), values)
	require.Error(t, err)
	require.False(t, isCaseEntityClientDBError(err))
}

func TestCaseEntitiesKnownIdentitiesSurviveUniqueIndexOutage(t *testing.T) {
	org := uuid.New()
	idField, emailField := "id-field", "email-field"
	idPivot, emailPivot := uuid.New(), uuid.New()
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "known"}
	dm := models.DataModel{Tables: map[string]models.Table{"customers": {ID: "table", Name: "customers", Fields: map[string]models.Field{
		"object_id": {ID: idField, Name: "object_id"}, "email": {ID: emailField, Name: "email"},
	}}}}
	values := []models.PivotDataWithCount{{PivotId: idPivot.String(), PivotValue: "known"}, {PivotId: emailPivot.String(), PivotValue: "alice@example.com"}}
	uc := IngestedDataReaderUsecase{dataModelUsecase: caseEntityModelStub{model: dm, uniqueError: repositories.ClientDatabaseError{Err: errors.New("pg_indexes query failed")}}, repository: caseEntityPivotsStub{pivots: []models.PivotMetadata{{Id: idPivot, BaseTableId: "table", FieldId: &idField}, {Id: emailPivot, BaseTableId: "table", FieldId: &emailField}}}, executorFactory: caseEntityExecutorStub{}}
	refs, err := uc.ReadCaseDecisionEntityRefs(context.Background(), org, values)
	require.Error(t, err)
	require.Equal(t, []models.CaseEntityRef{ref}, refs)
	// A direct object_id pivot never depends on ClientDB index introspection.
	direct, err := uc.ReadCaseDecisionEntityRefs(context.Background(), org, values[:1])
	require.NoError(t, err)
	require.Equal(t, []models.CaseEntityRef{ref}, direct)
	caseUC := CaseUseCase{ingestedDataReader: uc}
	manual := []models.CaseManualEntity{{CaseEntityRef: ref}}
	best, err := caseUC.assembleCaseEntities(context.Background(), org, manual, values, true)
	require.NoError(t, err)
	require.Len(t, best, 1)
	require.Equal(t, ref, best[0].CaseEntityRef)
	require.Equal(t, []models.CaseEntitySource{models.CaseEntitySourceManual, models.CaseEntitySourceDecision}, best[0].Sources)
	require.Nil(t, best[0].Data)
	_, err = caseUC.assembleCaseEntities(context.Background(), org, manual, values, false)
	require.Error(t, err)
	uc.dataModelUsecase = caseEntityModelStub{model: dm, uniqueError: models.ForbiddenError}
	caseUC.ingestedDataReader = uc
	_, err = caseUC.assembleCaseEntities(context.Background(), org, manual, values, true)
	require.ErrorIs(t, err, models.ForbiddenError)
	// Connection initialization also preserves direct decision identities after commit.
	uc.dataModelUsecase = caseEntityModelStub{model: dm}
	uc.executorFactory = caseEntityExecutorStub{connectionError: repositories.ClientDatabaseError{Err: errors.New("client unavailable")}}
	caseUC.ingestedDataReader = uc
	best, err = caseUC.assembleCaseEntities(context.Background(), org, nil, values[:1], true)
	require.NoError(t, err)
	require.Len(t, best, 1)
	require.Equal(t, ref, best[0].CaseEntityRef)
	require.Equal(t, []models.CaseEntitySource{models.CaseEntitySourceDecision}, best[0].Sources)
	require.Nil(t, best[0].Data)
	_, err = caseUC.assembleCaseEntities(context.Background(), org, nil, values[:1], false)
	require.Error(t, err)

}
