package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type pivotReaderRepository struct {
	ingestedDataReaderRepository
	pivots []models.PivotMetadata
}

func (r pivotReaderRepository) ListPivots(_ context.Context, _ repositories.Executor, _ uuid.UUID, _ *string, _ bool, includeDeleted bool) ([]models.PivotMetadata, error) {
	if !includeDeleted {
		panic("historical pivots must be included")
	}
	return r.pivots, nil
}

type pivotReaderModel struct{ model models.DataModel }

func (r pivotReaderModel) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return r.model, nil
}

func TestReadPivotObjectsFromValuesMissingData(t *testing.T) {
	for _, tc := range []struct {
		name    string
		field   *string
		links   []string
		deleted *time.Time
	}{
		{name: "unresolved field without path", field: utils.Ptr("missing-field")},
		{name: "deleted field without path", field: utils.Ptr("missing-field"), deleted: utils.Ptr(time.Now())},
		{name: "deleted link", links: []string{"missing-link"}, deleted: utils.Ptr(time.Now())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			uc := NewIngestedDataReaderUsecase(
				nil,
				pivotReaderRepository{pivots: []models.PivotMetadata{{
					Id: id, BaseTableId: "missing-table", FieldId: tc.field,
					PathLinkIds: tc.links, DeletedAt: tc.deleted,
				}}},
				executor_factory.NewExecutorFactoryStub(),
				pivotReaderModel{model: models.DataModel{}},
			)
			got, err := uc.ReadPivotObjectsFromValues(context.Background(), uuid.New(), []models.PivotDataWithCount{{PivotId: id.String(), PivotValue: "historical value", NbOfDecisions: 2}})
			require.NoError(t, err)
			require.Len(t, got, 1)
			require.Equal(t, models.PivotTypeField, got[0].PivotType)
			require.Equal(t, "historical value", got[0].PivotValue)
			require.Equal(t, 2, got[0].NumberOfDecisions)
			require.False(t, got[0].IsIngested)
		})
	}
}
