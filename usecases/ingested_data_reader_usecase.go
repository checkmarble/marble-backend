package usecases

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ingestedDataReaderClientDbRepository interface {
	ListIngestedObjects(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		params models.ExplorationOptions,
		cursorId *string,
		limit int,
		fieldsToRead ...string,
	) ([]models.DataModelObject, error)
	QueryIngestedObjectByUniqueField(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		uniqueFieldValue string,
		uniqueFieldName string,
		metadataFields ...string,
	) ([]models.DataModelObject, error)
	GatherFieldStatistics(ctx context.Context, exec repositories.Executor, table models.Table,
		orgId uuid.UUID) ([]models.FieldStatistics, error)
}

type ingestedDataReaderRepository interface {
	ListPivots(
		ctx context.Context,
		exec repositories.Executor,
		organizationId uuid.UUID,
		tableId *string,
		useCache bool,
		includeDeleted bool,
	) ([]models.PivotMetadata, error)
	GetEntityAnnotations(
		ctx context.Context,
		exec repositories.Executor,
		req models.EntityAnnotationRequest,
	) ([]models.EntityAnnotation, error)
}

type ingestedDataReaderDataModelUsecase interface {
	GetDataModel(ctx context.Context, organizationID uuid.UUID, options models.DataModelReadOptions,
		useCache bool) (models.DataModel, error)
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
	organizationId uuid.UUID,
	dataModel *models.DataModel,
	objectType string,
	uniqueFieldValue string,
	uniqueFieldName string,
) ([]models.ClientObjectDetail, error) {
	if dataModel == nil {
		d, err := usecase.dataModelUsecase.GetDataModel(ctx, organizationId, models.DataModelReadOptions{}, true)
		if err != nil {
			return nil, err
		}
		dataModel = &d
	}

	table, ok := dataModel.Tables[objectType]
	if !ok {
		return nil, errors.Wrapf(models.NotFoundError, "Table '%s' not found in GetIngestedObject", objectType)
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, err
	}

	objects, err := usecase.clientDbRepository.QueryIngestedObjectByUniqueField(
		ctx,
		db,
		table,
		uniqueFieldValue,
		uniqueFieldName,
		"valid_from",
	)
	if err != nil {
		return nil, err
	}

	clientObjects := make([]models.ClientObjectDetail, len(objects))
	for i, object := range objects {
		validFrom, _ := object.Metadata["valid_from"].(time.Time)
		clientObject := models.ClientObjectDetail{
			Data:     object.Data,
			Metadata: models.ClientObjectMetadata{ValidFrom: &validFrom, ObjectType: objectType},
		}
		clientObjects[i] = clientObject
	}
	return clientObjects, nil
}

func (usecase IngestedDataReaderUsecase) ReadPivotObjectsFromValues(
	ctx context.Context,
	orgId uuid.UUID,
	values []models.PivotDataWithCount,
) ([]models.PivotObject, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	dataModel, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeUnicityConstraints: true,
	}, true)
	if err != nil {
		return nil, err
	}

	// Include soft-deleted pivots: the input values come from decisions, which keep
	// referencing their pivot_id after the pivot is deleted.
	pivotsMeta, err := usecase.repository.ListPivots(ctx, exec, orgId, nil, true, true)
	if err != nil {
		return nil, err
	}
	pivotIds := make(map[string]struct{}, len(values))
	for _, value := range values {
		pivotIds[value.PivotId] = struct{}{}
	}
	pivots := make([]models.Pivot, 0, len(pivotsMeta))
	for _, pivot := range pivotsMeta {
		if _, ok := pivotIds[pivot.Id.String()]; !ok {
			continue
		}
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

		// A pivot (typically soft-deleted) whose path references a link that no longer
		// exists cannot be resolved to a table and field anymore. Degrade it to a plain
		// value so decisions referencing it still render, without ingested data.
		// see: enrichPivotObjectWithData
		if t == models.PivotTypeObject && (pivot.PivotTable == "" || fieldName == "") {
			logger.WarnContext(ctx,
				"Pivot references a link that no longer exists, pivot objects will not be enriched",
				"pivotId", pivot.Id)
			t = models.PivotTypeField
		}

		mapOfPivotDetail[pivot.Id.String()] = pivotObjectDetail{
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
			PivotValue:        value.PivotValue,
			PivotId:           value.PivotId,
			PivotType:         pivotDetail.pivotType,
			PivotObjectName:   pivotDetail.pivotTable,
			PivotFieldName:    pivotDetail.pivotField,
			PivotObjectData:   models.ClientObjectDetail{Data: map[string]any{}},
			NumberOfDecisions: value.NbOfDecisions,
		}
		if pivotDetail.pivotField != "" {
			pivotObject.PivotObjectData.Data[pivotDetail.pivotField] = value.PivotValue
		}
		if pivotDetail.pivotField == "object_id" {
			pivotObject.PivotObjectId = value.PivotValue
		}
		// Best effort: a pivot object that cannot be enriched (e.g. its pivot definition
		// references data model elements that no longer exist) is still returned with the
		// bare pivot value, and must not fail the whole batch.
		enrichedPivotObject, err := usecase.enrichPivotObjectWithData(ctx, pivotObject, orgId, dataModel)
		if err != nil {
			logger.WarnContext(ctx,
				"failed to read data for pivot object in ReadPivotObjectsFromValues, returning it without enrichment",
				"pivotId", pivotObject.PivotId, "pivotValue", pivotObject.PivotValue, "error", err.Error())
		} else {
			pivotObject = enrichedPivotObject
		}
		pivotObjectsMap[pivotObjectsMapKey(pivotDetail.pivotTable, pivotDetail.pivotField, value.PivotValue)] = pivotObject
	}

	pivotObjectsAsSlice := make([]models.PivotObject, 0, len(pivotObjectsMap))
	for _, pivotObject := range pivotObjectsMap {
		if pivotObject.PivotType != models.PivotTypeObject || pivotObject.PivotObjectId != "" {
			pivotObjectsAsSlice = append(pivotObjectsAsSlice, pivotObject)
		}
	}
	return pivotObjectsAsSlice, nil
}

func (usecase IngestedDataReaderUsecase) enrichPivotObjectWithData(
	ctx context.Context,
	pivotObject models.PivotObject,
	organizationId uuid.UUID,
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

	if pivotObject.PivotObjectId == "" {
		if objectId, ok := objectData.Data["object_id"].(string); ok {
			pivotObject.PivotObjectId = objectId
		}
	}

	// Annotations don't depend on the existence of the pivot object in IngestedData, so we can get them first
	// to enrich the pivot object data
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
	pivotObject.PivotObjectData.Annotations = models.GroupAnnotationsByType(annotations)

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
	orgId uuid.UUID,
	objectType string,
	input models.ClientDataListRequestBody,
	fieldsToRead ...string,
) (objects []models.ClientObjectDetail, fieldStats []models.FieldStatistics, pagination models.ClientDataListPagination, err error) {
	dataModel, err := usecase.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeNavigationOptions: true,
	}, true)
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
			"Source table '%s' not found in ReadIngestedClientObjects", explo.SourceTableName)
		return
	}
	filterField, ok := targetTable.Fields[explo.FilterFieldName]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Field '%s' not found in table '%s' in ReadIngestedClientObjects",
			explo.FilterFieldName, targetTable.Name)
		return
	}
	_, ok = targetTable.Fields[explo.OrderingFieldName]
	if !ok {
		err = errors.Wrapf(models.NotFoundError,
			"Field '%s' not found in table '%s' in ReadIngestedClientObjects",
			explo.OrderingFieldName, targetTable.Name)
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

	// The repository must only ever receive validated and typed filters
	input.ExplorationOptions.Filters, err = validateExplorationFilters(targetTable, explo.Filters)
	if err != nil {
		return
	}

	// All input validation having passed, now query the objects for real
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return
	}

	fieldStats, err = usecase.clientDbRepository.GatherFieldStatistics(ctx, db, targetTable, orgId)
	if err != nil {
		return
	}

	rawObjects, err := usecase.clientDbRepository.ListIngestedObjects(ctx, db, targetTable,
		input.ExplorationOptions, input.CursorId, input.Limit+1)
	if err != nil {
		return
	}
	if len(rawObjects) == 0 {
		return nil, nil, models.ClientDataListPagination{
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
			Metadata: models.ClientObjectMetadata{ValidFrom: &validFrom, ObjectType: objectType},
		}

		var annotations []models.EntityAnnotation
		annotations, err = usecase.repository.GetEntityAnnotations(
			ctx,
			usecase.executorFactory.NewExecutor(),
			models.EntityAnnotationRequest{
				OrgId:      orgId,
				ObjectType: objectType,
				ObjectId:   object.Data["object_id"].(string),
			})
		if err != nil {
			return
		}
		clientObject.Annotations = models.GroupAnnotationsByType(annotations)

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

	return objects, fieldStats, pagination, nil
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
	organizationId uuid.UUID,
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

const (
	explorationMaxFilters        = 20
	explorationMaxFilterInValues = 100
)

// explorationFilterRule gives the field types a filter operator applies to, and how many values it takes.
type explorationFilterRule struct {
	dataTypes []models.DataType
	maxValues int
}

var explorationFilterRules = map[models.ExplorationFilterOperator]explorationFilterRule{
	models.ExplorationFilterContains: {[]models.DataType{models.String, models.IpAddress}, 1},
	models.ExplorationFilterIn: {
		[]models.DataType{models.String, models.Int, models.Float},
		explorationMaxFilterInValues,
	},
	models.ExplorationFilterGreaterOrEqual: {[]models.DataType{models.Int, models.Float, models.Timestamp}, 1},
	models.ExplorationFilterLesserOrEqual:  {[]models.DataType{models.Int, models.Float, models.Timestamp}, 1},
	models.ExplorationFilterLesser:         {[]models.DataType{models.Int, models.Float, models.Timestamp}, 1},
}

// validateExplorationFilters checks user-provided filters against the target table, and returns them with their
// values coerced to the Go type matching the field data type. An unknown field is a NotFoundError, as for the other
// fields of the exploration options; all other failures are UnprocessableEntityError.
func validateExplorationFilters(table models.Table, filters []models.ExplorationFilter) ([]models.ExplorationFilter, error) {
	if len(filters) > explorationMaxFilters {
		return nil, errors.Wrapf(models.UnprocessableEntityError,
			"too many filters: %d, maximum is %d", len(filters), explorationMaxFilters)
	}

	out := make([]models.ExplorationFilter, len(filters))
	for i, f := range filters {
		field, ok := table.Fields[f.FieldName]
		if !ok {
			return nil, errors.Wrapf(models.NotFoundError,
				"Field '%s' not found in table '%s' in ReadIngestedClientObjects", f.FieldName, table.Name)
		}
		invalid := func(format string, args ...any) error {
			return errors.Wrapf(models.UnprocessableEntityError, "invalid filter '%s' on field '%s' of type %s: %s",
				f.Operator, f.FieldName, field.DataType, fmt.Sprintf(format, args...))
		}

		rule, ok := explorationFilterRules[f.Operator]
		if !ok {
			return nil, invalid("unknown operator")
		}
		if !slices.Contains(rule.dataTypes, field.DataType) {
			return nil, invalid("operator not supported for this field type")
		}
		if len(f.Values) < 1 || len(f.Values) > rule.maxValues {
			return nil, invalid("expected between 1 and %d values, got %d", rule.maxValues, len(f.Values))
		}

		// A comparison takes a single value. On an Int field, that bound can be any number: it may have to move, and
		// the operator with it, to stay within what the column can be compared to.
		isComparison := f.Operator == models.ExplorationFilterGreaterOrEqual ||
			f.Operator == models.ExplorationFilterLesserOrEqual || f.Operator == models.ExplorationFilterLesser
		if field.DataType == models.Int && isComparison {
			bound, isNumber := f.Values[0].(float64)
			if !isNumber {
				return nil, invalid("value must be a number")
			}
			operator, intBound := normalizeIntBound(f.Operator, bound)
			out[i] = models.ExplorationFilter{FieldName: f.FieldName, Operator: operator, Values: []any{intBound}}
			continue
		}

		values := make([]any, len(f.Values))
		for j, v := range f.Values {
			value, err := coerceExplorationFilterValue(field.DataType, f.Operator, v)
			if err != nil {
				return nil, invalid("%s", err)
			}
			values[j] = value
		}
		out[i] = models.ExplorationFilter{FieldName: f.FieldName, Operator: f.Operator, Values: values}
	}

	return out, nil
}

// normalizeIntBound returns the operator and the bound to use to compare an Int field, stored as a postgres INTEGER,
// with any number:
//   - a fractional bound moves to the nearest integer giving the same result (">= 1.5" becomes ">= 2")
//   - a bound outside of the range of the column cannot be sent to postgres. No value is past it, so the comparison
//     matches either every non null value or none: it becomes the equivalent ">= MinInt32" or "< MinInt32".
func normalizeIntBound(
	operator models.ExplorationFilterOperator,
	bound float64,
) (models.ExplorationFilterOperator, int64) {
	if operator == models.ExplorationFilterLesserOrEqual {
		bound = math.Floor(bound)
	} else {
		bound = math.Ceil(bound)
	}
	if bound >= math.MinInt32 && bound <= math.MaxInt32 {
		return operator, int64(bound)
	}

	isLowerBound := operator == models.ExplorationFilterGreaterOrEqual
	if matchesAll := (bound > math.MaxInt32) != isLowerBound; matchesAll {
		return models.ExplorationFilterGreaterOrEqual, math.MinInt32
	}
	return models.ExplorationFilterLesser, math.MinInt32
}

// coerceExplorationFilterValue converts a filter value, received as a string or a float64, to the Go type matching
// the field data type.
func coerceExplorationFilterValue(
	dataType models.DataType,
	operator models.ExplorationFilterOperator,
	value any,
) (any, error) {
	str, isString := value.(string)
	num, isNumber := value.(float64)

	switch dataType {
	case models.String, models.IpAddress:
		switch {
		case !isString:
			return nil, errors.New("value must be a string")
		case operator == models.ExplorationFilterContains && str == "":
			return nil, errors.New("value must not be empty")
		case strings.ContainsRune(str, 0):
			// Postgres text cannot hold a NUL character: the query would fail
			return nil, errors.New("value must not contain a NUL character")
		}
		return str, nil
	case models.Timestamp:
		t, err := time.Parse(time.RFC3339, str)
		if !isString || err != nil {
			return nil, errors.New("value must be an RFC3339 timestamp")
		}
		return t, nil
	case models.Float:
		if !isNumber {
			return nil, errors.New("value must be a number")
		}
		return num, nil
	case models.Int:
		// Int fields are stored as postgres INTEGER. Only the values of an "in" filter get here: the bound of a
		// comparison is more lenient, see normalizeIntBound.
		switch {
		case !isNumber:
			return nil, errors.New("value must be a number")
		case num != math.Trunc(num):
			return nil, errors.New("value must be an integer")
		case num < math.MinInt32 || num > math.MaxInt32:
			return nil, errors.New("value out of range")
		}
		return int64(num), nil
	}
	return nil, errors.New("unsupported field type")
}
