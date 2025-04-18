package usecases

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type ingestedDataReaderClientDbRepository interface {
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
	ListPivots(
		ctx context.Context,
		exec repositories.Executor,
		organization_id string,
		tableId *string,
	) ([]models.PivotMetadata, error)
	GetEntityAnnotations(
		ctx context.Context,
		exec repositories.Executor,
		req models.EntityAnnotationRequest,
	) ([]models.EntityAnnotation, error)
}

type ingestedDataReaderDataModelUsecase interface {
	GetDataModel(ctx context.Context, organizationID string, options models.DataModelReadOptions) (models.DataModel, error)
}

type IngestedDataReaderUsecase struct {
	clientDbRepository ingestedDataReaderClientDbRepository
	repository         ingestedDataReaderRepository
	executorFactory    executor_factory.ExecutorFactory
	dataModelUsecase   ingestedDataReaderDataModelUsecase
}

func NewIngestedDataReaderUsecase(
	clientDbRepository ingestedDataReaderClientDbRepository,
	repository ingestedDataReaderRepository,
	executorFactory executor_factory.ExecutorFactory,
	dataModelUsecase ingestedDataReaderDataModelUsecase,
) IngestedDataReaderUsecase {
	return IngestedDataReaderUsecase{
		clientDbRepository: clientDbRepository,
		repository:         repository,
		executorFactory:    executorFactory,
		dataModelUsecase:   dataModelUsecase,
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
		d, err := usecase.dataModelUsecase.GetDataModel(ctx, organizationId, models.DataModelReadOptions{})
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

	clientObjects := make([]models.ClientObjectDetail, len(objects))
	for i, object := range objects {
		validFrom, _ := object.Metadata["valid_from"].(time.Time)
		clientObject := models.ClientObjectDetail{
			Data:     object.Data,
			Metadata: models.ClientObjectMetadata{ValidFrom: validFrom, ObjectType: objectType},
		}
		clientObjects[i] = clientObject
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

	dataModel, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeUnicityConstraints: true,
	})
	if err != nil {
		return nil, err
	}

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

	// keys of format "{tableName}.{fieldName}.{pivotValue}" - we want to group further than just by pivotId-pivotValue, to deduplicate pivot objects that appear from different pivots (trigger object types)
	pivotObjectsMapKey := func(table, field, value string) string {
		return table + "." + field + "." + value
	}

	pivotObjectsMap := make(map[string]models.PivotObject, len(values))
	for _, value := range values {
		pivotDetail, ok := mapOfPivotDetail[value.PivotId]
		if !ok {
			logger.WarnContext(ctx, "Pivot unexpectedly not found in map in ReadPivotObjectsFromValues", "pivotId", value.PivotId)
			continue
		}

		if pivotObject, ok := pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable,
			pivotDetail.pivotField, value.PivotValue)]; ok {
			pivotObject.NumberOfDecisions += value.NbOfDecisions
			pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable,
				pivotDetail.pivotField, value.PivotValue)] = pivotObject
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
		pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField, value.PivotValue)] = pivotObject
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

	annotations, err := usecase.repository.GetEntityAnnotations(
		ctx,
		usecase.executorFactory.NewExecutor(),
		models.EntityAnnotationRequest{
			OrgId:      organizationId,
			ObjectType: pivotObject.PivotObjectName,
			ObjectId:   pivotObject.PivotObjectId,
		})
	if err != nil {
		return models.PivotObject{}, err
	}
	pivotObject.Annotations = models.GroupAnnotationsByType(annotations)

	// Enriches the pivot object with one level of related objects (fiend objects that are linked to the pivot object, without further recursion)
	pivotObject.PivotObjectData, err = usecase.enrichClientDataObjectWithRelatedObjectsData(
		ctx,
		organizationId,
		dataModel.Tables[pivotObject.PivotObjectName],
		dataModel,
		pivotObject.PivotObjectData,
	)
	if err != nil {
		return models.PivotObject{}, err
	}

	return pivotObject, nil
}

func (usecase IngestedDataReaderUsecase) ReadIngestedClientObjects(
	ctx context.Context,
	orgId string,
	objectType string,
	input models.ClientDataListRequestBody,
) (objects []models.ClientObjectDetail, pagination models.ClientDataListPagination, err error) {
	dataModel, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeNavigationOptions: true,
	})
	if err != nil {
		return
	}

	targetTable, ok := dataModel.Tables[objectType]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Table '%s' not found in ReadIngestedClientObjects", objectType)
		return
	}

	explo := input.ExplorationOptions
	sourceTable, ok := dataModel.Tables[explo.SourceTableName]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Table '%s' not found in ReadIngestedClientObjects", explo.SourceTableName)
		return
	}
	filterField, ok := targetTable.Fields[explo.FilterFieldName]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Field '%s' not found in table '%s' in ReadIngestedClientObjects",
			explo.FilterFieldName, explo.SourceTableName)
		return
	}
	_, ok = targetTable.Fields[explo.OrderingFieldName]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Field '%s' not found in table '%s' in ReadIngestedClientObjects",
			explo.OrderingFieldName, explo.SourceTableName)
		return
	}
	navigationOptionFound := false
	for _, options := range sourceTable.NavigationOptions {
		if options.FilterFieldName == explo.FilterFieldName &&
			options.OrderingFieldName == explo.OrderingFieldName &&
			options.TargetTableName == targetTable.Name {
			navigationOptionFound = true
			break
		}
	}
	if !navigationOptionFound {
		err = errors.Wrapf(models.UnprocessableEntityError,
			"Navigation option not found allowed from table %s => table %s filtering on %s ordering on %s",
			sourceTable.Name, targetTable.Name, explo.FilterFieldName, explo.OrderingFieldName)
		return
	}
	switch filterField.DataType {
	case models.String:
		if explo.FilterFieldValue.StringValue == nil {
			err = errors.Wrapf(models.UnprocessableEntityError,
				"Filter field %s of type %s must be a string in ReadIngestedClientObjects",
				explo.FilterFieldName, filterField.DataType)
			return
		}
	case models.Float:
		if explo.FilterFieldValue.FloatValue == nil {
			err = errors.Wrapf(models.UnprocessableEntityError,
				"Filter field %s of type %s must be a number in ReadIngestedClientObjects",
				explo.FilterFieldName, filterField.DataType)
			return
		}
	default:
		err = errors.Wrapf(models.UnprocessableEntityError,
			"Filter field %s of type %s not supported in ReadIngestedClientObjects",
			explo.FilterFieldName, filterField.DataType)
		return
	}

	// All input validation having passed, now query the objects for real
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return
	}

	rawObjects, err := usecase.clientDbRepository.ListIngestedObjects(ctx, db, targetTable,
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

		clientObject, err = usecase.enrichClientDataObjectWithRelatedObjectsData(
			ctx,
			orgId,
			targetTable,
			dataModel,
			clientObject,
			allParentTables(dataModel, sourceTable.Name)...,
		)
		if err != nil {
			return
		}
		objects = append(objects, clientObject)
	}

	return objects, pagination, nil
}

func allParentTables(dataModel models.DataModel, tableName string) []string {
	visited := map[string]bool{tableName: true}
	result := []string{tableName}
	queue := []string{tableName}

	for len(queue) > 0 {
		// Pop first element from queue
		current := queue[0]
		queue = queue[1:]

		currentTable := dataModel.Tables[current]
		for _, link := range currentTable.LinksToSingle {
			if !visited[link.ParentTableName] {
				visited[link.ParentTableName] = true
				result = append(result, link.ParentTableName)
				queue = append(queue, link.ParentTableName)
			}
		}
	}

	return result
}

// Enriches a client data object with one level (no further recursion) of related objects, following links pointing from that object.
// Optionally takes a list of tables to which NOT to go, so that in a context where we are e.g. listing transactions from an account,
// we may return data on a transaction.beneficiary but don't lose time reading data on transaction.account (which is presumably already in the context)
func (usecase IngestedDataReaderUsecase) enrichClientDataObjectWithRelatedObjectsData(
	ctx context.Context,
	organizationId string,
	baseObjectTable models.Table,
	dataModel models.DataModel,
	baseObject models.ClientObjectDetail,
	parentObjectsToIgnore ...string,
) (models.ClientObjectDetail, error) {
	for _, link := range baseObjectTable.LinksToSingle {
		if slices.Contains(parentObjectsToIgnore, link.ParentTableName) {
			continue
		}

		relatedObjectUniqueField := link.ParentFieldName
		relatedObjectObjectType := link.ParentTableName
		linkValue, ok := baseObject.Data[link.ChildFieldName]
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
			return models.ClientObjectDetail{}, errors.Wrapf(err,
				"failed to read related object data of type %s through link %s for object %s: %s",
				relatedObjectObjectType, link.Name, baseObjectTable.Name, relatedObjectUniqueField)
		}
		if len(relatedObjectDataSlice) == 0 {
			continue
		}
		relatedObjectData := relatedObjectDataSlice[0]
		baseObject.RelatedObjects = append(
			baseObject.RelatedObjects, models.RelatedObject{
				LinkName: link.Name,
				Detail: models.ClientObjectDetail{
					Data:     relatedObjectData.Data,
					Metadata: relatedObjectData.Metadata,
				},
			})
	}
	return baseObject, nil
}
