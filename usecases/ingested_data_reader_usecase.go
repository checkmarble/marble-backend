package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type ingestedDataReaderClientDbRepository interface {
	QueryIngestedObject(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		objectId string,
	) ([]models.DataModelObject, error)
	ListAllObjectIdsFromTable(
		ctx context.Context,
		exec repositories.Executor,
		tableName string,
		filters ...models.Filter,
	) ([]string, error) // TODO: remove this, only introduced for a MVP for Chris
}

type ingestedDataReaderRepository interface {
	GetDataModel(
		ctx context.Context,
		exec repositories.Executor,
		organizationID string,
		fetchEnumValues bool,
	) (models.DataModel, error)
	ListPivots(
		ctx context.Context,
		exec repositories.Executor,
		organization_id string,
		tableId *string,
	) ([]models.PivotMetadata, error)
}

type IngestedDataReaderUsecase struct {
	clientDbRepository ingestedDataReaderClientDbRepository
	repository         ingestedDataReaderRepository
	executorFactory    executor_factory.ExecutorFactory
}

func NewIngestedDataReaderUsecase(
	clientDbRepository ingestedDataReaderClientDbRepository,
	repository ingestedDataReaderRepository,
	executorFactory executor_factory.ExecutorFactory,
) IngestedDataReaderUsecase {
	return IngestedDataReaderUsecase{
		clientDbRepository: clientDbRepository,
		repository:         repository,
		executorFactory:    executorFactory,
	}
}

func (usecase IngestedDataReaderUsecase) GetIngestedObject(
	ctx context.Context,
	organizationId string,
	objectType string,
	objectId string,
) ([]models.DataModelObject, error) {
	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.repository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return nil, err
	}

	table := dataModel.Tables[objectType]

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	return usecase.clientDbRepository.QueryIngestedObject(ctx, db, table, objectId)
}

// TODO: logic to be merged with the method above. Rework function signatures to use DataModelObject=>ClientObjectDetail everywhere
func (usecase IngestedDataReaderUsecase) GetIngestedObject_variant(
	ctx context.Context,
	organizationId string,
	dataModel models.DataModel,
	objectType string,
	objectId string,
) ([]models.ClientObjectDetail, error) {
	table := dataModel.Tables[objectType]

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	objects, err := usecase.clientDbRepository.QueryIngestedObject(ctx, db, table, objectId)
	if err != nil {
		return nil, err
	}

	clientObjects := make([]models.ClientObjectDetail, 0, len(objects))
	for _, object := range objects {
		validFrom, _ := object.Metadata["valid_from"].(time.Time)
		clientObject := models.ClientObjectDetail{
			Data:     object.Data,
			Metadata: models.ClientObjectMetadata{ValidFrom: validFrom, ObjectType: objectType},
		}
		clientObjects = append(clientObjects, clientObject)
	}
	return clientObjects, nil
}

func (usecase IngestedDataReaderUsecase) ReadPivotObjectsFromValues(
	ctx context.Context,
	orgId string,
	values []models.PivotDataWithCount,
) ([]models.PivotObject, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	dataModel, err := usecase.repository.GetDataModel(ctx, exec, orgId, false)
	if err != nil {
		return nil, err
	}

	pivotsMeta, err := usecase.repository.ListPivots(ctx, exec, orgId, nil)
	if err != nil {
		return nil, err
	}
	pivots := make([]models.Pivot, 0, len(pivotsMeta))
	for _, pivot := range pivotsMeta {
		pivots = append(pivots, models.AdaptPivot(pivot, dataModel))
	}
	type pivotObjectDetail struct {
		pivotTable string
		pivotField string
		pivotType  string // "object" or "field"
	}

	mapOfPivotDetail := make(map[string]pivotObjectDetail, len(pivots))
	for _, pivot := range pivots {
		var t string
		switch {
		case len(pivot.PathLinks) > 0 || pivot.Field == "object_id":
			t = "object"
		default:
			t = "field"
		}

		var field string
		switch {
		case pivot.Field != "":
			field = pivot.Field
		default:
			lastLink := dataModel.AllLinksAsMap()[pivot.PathLinkIds[len(pivot.PathLinkIds)-1]]
			lastField := dataModel.AllFieldsAsMap()[lastLink.ParentFieldId]
			field = lastField.Name
		}
		mapOfPivotDetail[pivot.Id] = pivotObjectDetail{
			pivotTable: pivot.PivotTable,
			pivotField: field,
			pivotType:  t,
		}
	}

	// keys of format "tableName.fieldName" - we want to group further than just by pivotId-pivotValue
	pivotObjectsMapKey := func(table, field string) string {
		return table + "." + field
	}

	pivotObjects := make(map[string]models.PivotObject, len(values))
	for _, value := range values {
		pivotDetail, ok := mapOfPivotDetail[value.PivotId]
		if !ok {
			logger.WarnContext(ctx, "Pivot unexpectedly not found in map in ReadPivotObjectsFromValues", "pivotId", value.PivotId)
			continue
		}
		pivotObject := models.PivotObject{
			PivotValue:      value.PivotValue,
			PivotId:         value.PivotId,
			PivotType:       pivotDetail.pivotType,
			PivotObjectName: pivotDetail.pivotTable,
			PivotFieldName:  pivotDetail.pivotField,
			PivotObjectData: models.ClientObjectDetail{
				Data: map[string]any{
					pivotDetail.pivotField: value.PivotValue,
				},
			},
			NumberOfDecisions: value.NbOfDecisions,
		}
		if pivotDetail.pivotField == "object_id" {
			pivotObject.PivotObjectId = value.PivotValue
		}

		pivotObject, err = usecase.enrichPivotObjectWithData(ctx, pivotObject, orgId, dataModel)
		if err != nil {
			return nil, errors.Wrapf(err,
				"failed to read data for pivot object {id: %s, value: %s} in ReadPivotObjectsFromValues",
				pivotObject.PivotId, pivotObject.PivotValue)
		}
		// TODO tomorrow: finish the proper logic to not overwrite the pivotObject if it already exists
		pivotObjects[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField)] = pivotObject
	}

	pivotObjectsAsSlice := make([]models.PivotObject, 0, len(pivotObjects))
	for _, pivotObject := range pivotObjects {
		pivotObjectsAsSlice = append(pivotObjectsAsSlice, pivotObject)
	}
	return pivotObjectsAsSlice, nil
}

// TODO: add option to recursively fetch "nested" objects too
func (usecase IngestedDataReaderUsecase) enrichPivotObjectWithData(
	ctx context.Context,
	pivotObject models.PivotObject,
	organizationId string,
	dataModel models.DataModel,
) (models.PivotObject, error) {
	if pivotObject.PivotObjectId == "" {
		return pivotObject, nil
	}

	objectDataSlice, err := usecase.GetIngestedObject_variant(
		ctx,
		organizationId,
		dataModel,
		pivotObject.PivotObjectName,
		pivotObject.PivotObjectId)
	if err != nil {
		return models.PivotObject{}, err
	}
	if len(objectDataSlice) == 0 {
		pivotObject.PivotObjectData.Metadata = models.ClientObjectMetadata{
			ObjectType: pivotObject.PivotObjectName,
		}
		return pivotObject, nil
	}
	objectData := objectDataSlice[0]

	pivotObject.PivotObjectData.Data = objectData.Data
	pivotObject.PivotObjectData.Metadata = objectData.Metadata
	pivotObject.PivotObjectData.RelatedObjects = objectData.RelatedObjects
	pivotObject.IsIngested = true

	return pivotObject, nil
}

func (usecase IngestedDataReaderUsecase) ReadIngestedClientObjects(
	ctx context.Context,
	orgId string,
	objectType string,
) ([]models.ClientObjectDetail, bool, error) {
	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.repository.GetDataModel(ctx, exec, orgId, false)
	if err != nil {
		return nil, false, err
	}
	table, ok := dataModel.Tables[objectType]
	if !ok {
		return nil, false, errors.Wrapf(models.NotFoundError,
			"Table '%s' not found in ReadIngestedClientObjects", objectType)
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return nil, false, err
	}

	listObjectIds, err := usecase.clientDbRepository.ListAllObjectIdsFromTable(ctx, db, table.Name)
	if err != nil {
		return nil, false, err
	}

	if len(listObjectIds) == 0 {
		return nil, false, nil
	}

	objects, err := usecase.GetIngestedObject_variant(ctx, orgId, dataModel, objectType, listObjectIds[0])
	if err != nil {
		return nil, false, err
	}
	return append(objects, objects[0], objects[0], objects[0]), len(listObjectIds) > 1, nil
}
