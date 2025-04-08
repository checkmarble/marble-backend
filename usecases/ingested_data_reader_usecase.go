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
	ListIngestedObjects(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		params models.ExplorationOptions,
		cursorId *string,
		limit int,
	) ([]models.DataModelObject, error)
	QueryIngestedObjectByUniqueField(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		uniqueFieldValue string,
		uniqueFieldName string,
	) ([]models.DataModelObject, error)
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

type ingestedDataReaderIndexReader interface {
	ListAllUniqueIndexes(ctx context.Context, organizationId string) ([]models.UnicityIndex, error)
}

type IngestedDataReaderUsecase struct {
	clientDbRepository            ingestedDataReaderClientDbRepository
	repository                    ingestedDataReaderRepository
	executorFactory               executor_factory.ExecutorFactory
	ingestedDataReaderIndexReader ingestedDataReaderIndexReader
}

func NewIngestedDataReaderUsecase(
	clientDbRepository ingestedDataReaderClientDbRepository,
	repository ingestedDataReaderRepository,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReaderIndexReader ingestedDataReaderIndexReader,
) IngestedDataReaderUsecase {
	return IngestedDataReaderUsecase{
		clientDbRepository:            clientDbRepository,
		repository:                    repository,
		executorFactory:               executorFactory,
		ingestedDataReaderIndexReader: ingestedDataReaderIndexReader,
	}
}

func (usecase IngestedDataReaderUsecase) GetIngestedObject(
	ctx context.Context,
	organizationId string,
	dataModel *models.DataModel,
	objectType string,
	uniqueFieldValue string,
	uniqueFieldName string,
) ([]models.ClientObjectDetail, error) {
	if dataModel == nil {
		d, err := usecase.repository.GetDataModel(ctx,
			usecase.executorFactory.NewExecutor(), organizationId, false)
		if err != nil {
			return nil, err
		}
		dataModel = &d
	}

	table := dataModel.Tables[objectType]

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	objects, err := usecase.clientDbRepository.QueryIngestedObjectByUniqueField(ctx, db, table, uniqueFieldValue, uniqueFieldName)
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

	uniqueIndexes, err := usecase.ingestedDataReaderIndexReader.ListAllUniqueIndexes(ctx, orgId)
	if err != nil {
		return nil, err
	}
	dataModel = dataModel.AddUnicityConstraintStatusToDataModel(uniqueIndexes)

	pivotsMeta, err := usecase.repository.ListPivots(ctx, exec, orgId, nil)
	if err != nil {
		return nil, err
	}
	pivots := make([]models.Pivot, 0, len(pivotsMeta))
	for _, pivot := range pivotsMeta {
		pivots = append(pivots, pivot.Enrich(dataModel))
	}

	type pivotObjectDetail struct {
		pivotTable string
		pivotField string
		pivotType  models.PivotType
	}
	mapOfPivotDetail := make(map[string]pivotObjectDetail, len(pivots))
	for _, pivot := range pivots {
		var t models.PivotType
		pivotField := dataModel.AllFieldsAsMap()[pivot.FieldId]
		switch {
		case len(pivot.PathLinks) > 0 || pivotField.UnicityConstraint == models.ActiveUniqueConstraint:
			t = models.PivotTypeObject
		default:
			t = models.PivotTypeField
		}

		var fieldName string
		switch {
		case pivot.Field != "":
			fieldName = pivot.Field
		default:
			lastLink := dataModel.AllLinksAsMap()[pivot.PathLinkIds[len(pivot.PathLinkIds)-1]]
			lastField := dataModel.AllFieldsAsMap()[lastLink.ParentFieldId]
			fieldName = lastField.Name
		}
		mapOfPivotDetail[pivot.Id] = pivotObjectDetail{
			pivotTable: pivot.PivotTable,
			pivotField: fieldName,
			pivotType:  t,
		}
	}

	// keys of format "tableName.fieldName" - we want to group further than just by pivotId-pivotValue, to deduplicate pivot objects that appear from different pivots (trigger object types)
	pivotObjectsMapKey := func(table, field string) string {
		return table + "." + field
	}

	pivotObjectsMap := make(map[string]models.PivotObject, len(values))
	for _, value := range values {
		pivotDetail, ok := mapOfPivotDetail[value.PivotId]
		if !ok {
			logger.WarnContext(ctx, "Pivot unexpectedly not found in map in ReadPivotObjectsFromValues", "pivotId", value.PivotId)
			continue
		}

		if pivotObject, ok := pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField)]; ok {
			pivotObject.NumberOfDecisions += value.NbOfDecisions
			pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField)] = pivotObject
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
		pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField)] = pivotObject
	}

	pivotObjectsAsSlice := make([]models.PivotObject, 0, len(pivotObjectsMap))
	for _, pivotObject := range pivotObjectsMap {
		pivotObjectsAsSlice = append(pivotObjectsAsSlice, pivotObject)
	}
	return pivotObjectsAsSlice, nil
}

func (usecase IngestedDataReaderUsecase) enrichPivotObjectWithData(
	ctx context.Context,
	pivotObject models.PivotObject,
	organizationId string,
	dataModel models.DataModel,
) (models.PivotObject, error) {
	if pivotObject.PivotType == models.PivotTypeField {
		return pivotObject, nil
	}

	objectDataSlice, err := usecase.GetIngestedObject(
		ctx,
		organizationId,
		&dataModel,
		pivotObject.PivotObjectName,
		pivotObject.PivotValue,
		pivotObject.PivotFieldName)
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
	pivotObject.IsIngested = true

	// Enriches the pivot object with one level of related objects (fiend objects that are linked to the pivot object, without further recursion)
	table := dataModel.Tables[pivotObject.PivotObjectName]
	for _, link := range table.LinksToSingle {
		relatedObjectUniqueField := link.ParentFieldName
		relatedObjectObjectType := link.ParentTableName
		linkValue, ok := objectData.Data[link.ChildFieldName]
		if !ok {
			continue
		}
		// Will not work with links that are using "number" type fields. I accept this for now, it's not something we really try to encourage anyway
		// and the possibility may just go away completely if we deprecate "unique" fields on tables.
		linkValueStr, ok := linkValue.(string)
		if !ok {
			continue
		}
		relatedObjectDataSlice, err := usecase.GetIngestedObject(
			ctx,
			organizationId,
			&dataModel,
			relatedObjectObjectType,
			linkValueStr,
			relatedObjectUniqueField)
		if err != nil {
			return models.PivotObject{}, err
		}
		if len(relatedObjectDataSlice) == 0 {
			continue
		}
		relatedObjectData := relatedObjectDataSlice[0]
		pivotObject.PivotObjectData.RelatedObjects = append(
			pivotObject.PivotObjectData.RelatedObjects, models.RelatedObject{
				LinkName: link.Name,
				Detail: models.ClientObjectDetail{
					Data:     relatedObjectData.Data,
					Metadata: relatedObjectData.Metadata,
				},
			})
	}

	return pivotObject, nil
}

func (usecase IngestedDataReaderUsecase) ReadIngestedClientObjects(
	ctx context.Context,
	orgId string,
	objectType string,
	input models.ClientDataListRequestBody,
) (objects []models.ClientObjectDetail, pagination models.ClientDataListPagination, err error) {
	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.repository.GetDataModel(ctx, exec, orgId, false)
	if err != nil {
		return
	}
	table, ok := dataModel.Tables[objectType]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Table '%s' not found in ReadIngestedClientObjects", objectType)
		return
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return
	}

	rawObjects, err := usecase.clientDbRepository.ListIngestedObjects(ctx, db, table,
		input.ExplorationOptions, input.CursorId, input.Limit+1)
	if err != nil {
		return
	}
	if len(rawObjects) == 0 {
		return nil, models.ClientDataListPagination{
			HasNextPage: false,
		}, nil
	}
	hasNextPage := len(rawObjects) > input.Limit
	rawObjects = rawObjects[:min(input.Limit, len(rawObjects))]
	var nextCursor *string
	if hasNextPage {
		nextCursorVal := rawObjects[len(rawObjects)-1].Data["object_id"]
		nextCursorStr, _ := nextCursorVal.(string)
		nextCursor = &nextCursorStr
	}
	pagination = models.ClientDataListPagination{
		HasNextPage:  hasNextPage,
		NextCursorId: nextCursor,
	}

	for _, object := range rawObjects {
		validFrom, _ := object.Metadata["valid_from"].(time.Time)
		clientObject := models.ClientObjectDetail{
			Data:     object.Data,
			Metadata: models.ClientObjectMetadata{ValidFrom: validFrom, ObjectType: objectType},
		}
		objects = append(objects, clientObject)
	}

	return objects, pagination, nil
}
