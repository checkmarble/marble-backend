package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type caseEntityModelStub struct {
	ingestedDataReaderDataModelUsecase
	model models.DataModel
}

func (s caseEntityModelStub) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return s.model, nil
}

type caseEntityExecutorStub struct {
	executor_factory.ExecutorFactory
}

func (s caseEntityExecutorStub) NewClientDbExecutor(context.Context, uuid.UUID) (repositories.Executor, error) {
	return nil, nil
}

type caseEntityObjectsStub struct {
	ingestedDataReaderClientDbRepository
	objects []models.DataModelObject
	reads   int
}

func (s *caseEntityObjectsStub) QueryIngestedObjectByUniqueField(context.Context, repositories.Executor, models.Table, string, string, ...string) ([]models.DataModelObject, error) {
	return s.objects, nil
}
func (s *caseEntityObjectsStub) QueryIngestedObjectsByIds(context.Context, repositories.Executor, models.Table, []string) ([]models.DataModelObject, error) {
	s.reads++
	return s.objects, nil
}

func TestCaseEntityObjectValidation(t *testing.T) {
	ctx := context.Background()
	org := uuid.New()
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	for _, semantic := range []models.SemanticType{models.SemanticTypePerson, models.SemanticTypeCompany, models.SemanticTypePartner} {
		stub := &caseEntityObjectsStub{objects: []models.DataModelObject{{Data: map[string]any{"object_id": "c-123"}}}}
		uc := IngestedDataReaderUsecase{clientDbRepository: stub, dataModelUsecase: caseEntityModelStub{model: models.DataModel{Tables: map[string]models.Table{"customers": {Name: "customers", SemanticType: semantic}}}}, executorFactory: caseEntityExecutorStub{}}
		require.NoError(t, uc.RequireActiveCaseEntity(ctx, org, ref), semantic)
	}
	for _, semantic := range []models.SemanticType{models.SemanticTypeAccount, models.SemanticTypeTransaction, models.SemanticTypeOther} {
		stub := &caseEntityObjectsStub{objects: []models.DataModelObject{{Data: map[string]any{"object_id": "c-123"}}}}
		uc := IngestedDataReaderUsecase{clientDbRepository: stub, dataModelUsecase: caseEntityModelStub{model: models.DataModel{Tables: map[string]models.Table{"customers": {Name: "customers", SemanticType: semantic}}}}, executorFactory: caseEntityExecutorStub{}}
		require.Error(t, uc.RequireActiveCaseEntity(ctx, org, ref), semantic)
	}
	for _, objects := range [][]models.DataModelObject{nil, {{Data: map[string]any{"object_id": "c-123"}}, {Data: map[string]any{"object_id": "c-123"}}}} {
		stub := &caseEntityObjectsStub{objects: objects}
		uc := IngestedDataReaderUsecase{clientDbRepository: stub, dataModelUsecase: caseEntityModelStub{model: models.DataModel{Tables: map[string]models.Table{"customers": {Name: "customers", SemanticType: models.SemanticTypePerson}}}}, executorFactory: caseEntityExecutorStub{}}
		require.Error(t, uc.RequireActiveCaseEntity(ctx, org, ref))
	}
	missing := IngestedDataReaderUsecase{clientDbRepository: &caseEntityObjectsStub{}, dataModelUsecase: caseEntityModelStub{model: models.DataModel{Tables: map[string]models.Table{}}}, executorFactory: caseEntityExecutorStub{}}
	require.Error(t, missing.RequireActiveCaseEntity(ctx, org, ref))
}

func TestCaseEntityObjectBatchReadKeepsHistoricalReferences(t *testing.T) {
	stub := &caseEntityObjectsStub{objects: []models.DataModelObject{{Data: map[string]any{"object_id": "c-123", "name": "Alice"}}}}
	uc := IngestedDataReaderUsecase{clientDbRepository: stub, dataModelUsecase: caseEntityModelStub{model: models.DataModel{Tables: map[string]models.Table{"customers": {Name: "customers"}}}}, executorFactory: caseEntityExecutorStub{}}
	refs := []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}, {TableName: "customers", ObjectId: "missing"}, {TableName: "old_table", ObjectId: "old"}, {TableName: "customers", ObjectId: "c-123"}}
	got, err := uc.ReadCaseEntityObjects(context.Background(), uuid.New(), refs)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Alice", got[refs[0]].Data["name"])
	require.Equal(t, 1, stub.reads)
}
