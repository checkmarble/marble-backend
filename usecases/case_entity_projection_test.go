package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (s *CaseEntityReaderSuite) TestManualDecisionMerge() {
	field := "object-id-field"
	pivot := uuid.New()
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	table := models.Table{ID: "table", Name: ref.TableName, LinksToSingle: map[string]models.LinkToSingle{}, Fields: map[string]models.Field{"object_id": {ID: field, Name: "object_id", TableId: "table"}}}
	dm := models.DataModel{Tables: map[string]models.Table{table.Name: table}}
	s.expectModel(dm)
	s.expectModel(dm)
	s.model.On("ListPivots", s.ctx, s.exec, s.org, (*string)(nil), true, true).Return([]models.PivotMetadata{{Id: pivot, BaseTableId: table.ID, FieldId: &field}}, nil).Once()
	s.indexes.On("ListAllUniqueIndexes", s.ctx, s.org).Return([]models.UnicityIndex{}, nil).Once()
	s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
	s.objects.On("QueryIngestedObjectsByIds", s.ctx, s.exec, table, []string{ref.ObjectId}, []string(nil)).Return([]models.DataModelObject{{Data: map[string]any{"object_id": ref.ObjectId, "name": "Alice"}}}, nil).Once()
	uc := CaseUseCase{ingestedDataReader: s.reader}
	manual := []models.CaseManualEntity{{CaseEntityRef: ref}}
	got, err := uc.assembleCaseEntities(s.ctx, s.org, manual, []models.PivotDataWithCount{{PivotId: pivot.String(), PivotValue: ref.ObjectId}})
	s.NoError(err)
	s.Require().Len(got, 1)
	s.Equal([]models.CaseEntitySource{models.CaseEntitySourceManual, models.CaseEntitySourceDecision}, got[0].Sources)
	s.Equal("Alice", got[0].Data["name"])
	remaining := mergeCaseEntities(nil, []models.CaseEntityRef{ref}, nil)
	s.Equal([]models.CaseEntitySource{models.CaseEntitySourceDecision}, remaining[0].Sources)
	s.Nil(remaining[0].Data)
	s.NotNil(mergeCaseEntities(nil, nil, nil))
}
func (s *CaseEntityReaderSuite) TestHydrationFailureIsStrict() {
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	table := models.Table{Name: ref.TableName}
	s.expectModel(models.DataModel{Tables: map[string]models.Table{table.Name: table}})
	s.factory.On("NewClientDbExecutor", s.ctx, s.org).Return(s.exec, nil).Once()
	failure := errors.New("client query failed")
	s.objects.On("QueryIngestedObjectsByIds", s.ctx, s.exec, table, []string{ref.ObjectId}, []string(nil)).Return([]models.DataModelObject{}, failure).Once()
	uc := CaseUseCase{ingestedDataReader: s.reader}
	_, err := uc.assembleCaseEntities(s.ctx, s.org, []models.CaseManualEntity{{CaseEntityRef: ref}}, nil)
	s.ErrorIs(err, failure)
}
