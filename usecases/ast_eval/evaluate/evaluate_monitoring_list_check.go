package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const linkedTableCheckBatchSize = 1000

type MonitoringListCheckRepository interface {
	FindObjectRiskTopicsMetadata(
		ctx context.Context,
		exec repositories.Executor,
		filter models.ObjectRiskTopicsMetadataFilter,
	) ([]models.ObjectMetadata, error)
	ListPivots(
		ctx context.Context,
		exec repositories.Executor,
		organizationId uuid.UUID,
		tableId *string,
		useCache bool,
	) ([]models.PivotMetadata, error)
}

// ClientDbRepository provides access to client database operations for building navigation options during dry run.
type ClientDbRepository interface {
	ListAllIndexes(
		ctx context.Context,
		exec repositories.Executor,
		indexTypes ...models.IndexType,
	) ([]models.ConcreteIndex, error)
}

type MonitoringListCheck struct {
	ExecutorFactory executor_factory.ExecutorFactory

	OrgId              uuid.UUID
	ClientObject       models.ClientObject
	DataModel          models.DataModel
	Repository         MonitoringListCheckRepository
	IngestedDataReader repositories.IngestedDataReadRepository
	ClientDbRepository ClientDbRepository // For dry run navigation option validation
	ReturnFakeValue    bool
}

func (mlc MonitoringListCheck) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	// Get the configuration from arguments
	config, configErr := AdaptNamedArgument(
		arguments.NamedArgs,
		"config",
		adaptArgumentToJSONStruct[ast.MonitoringListCheckConfig],
	)
	if configErr != nil {
		return MakeEvaluateError(configErr)
	}

	// Validate the config
	if errs := mlc.validateMonitoringListCheckConfig(config); len(errs) > 0 {
		return nil, errs
	}

	// Step 1: fetch the ingested data based on config, get the `object_id` and query in object_risk_topics table if the element has a risk topic assigned
	hasRiskTopic, err := mlc.checkTargetObjectHasRiskTopic(ctx, config)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "failed to check target object risk topics"))
	}
	if hasRiskTopic {
		return true, nil
	}

	// Step 2: check LinkedTableChecks for risk topics
	// Process LinkToSingle checks first (simpler - single object lookup)
	for _, linkedCheck := range config.LinkedTableChecks {
		if linkedCheck.LinkToSingleName == nil {
			continue
		}
		hasRiskTopic, err := mlc.checkLinkedTableViaLinkToSingle(ctx, config, linkedCheck)
		if err != nil {
			return MakeEvaluateError(errors.Wrapf(err,
				"failed to check linked table %s", linkedCheck.TableName))
		}
		if hasRiskTopic {
			return true, nil
		}
	}

	// Then process NavigationOption checks (more complex - batch pagination)
	for _, linkedCheck := range config.LinkedTableChecks {
		if linkedCheck.NavigationOption == nil {
			continue
		}
		hasRiskTopic, err := mlc.checkLinkedTableViaNavigation(ctx, config, linkedCheck)
		if err != nil {
			return MakeEvaluateError(errors.Wrapf(err,
				"failed to check linked table %s", linkedCheck.TableName))
		}
		if hasRiskTopic {
			return true, nil
		}
	}

	return false, nil
}

// checkTargetObjectHasRiskTopic checks if the target object has any matching risk topics.
// It fetches the object_id from the target table using PathToTarget, then queries object_risk_topics.
func (mlc MonitoringListCheck) checkTargetObjectHasRiskTopic(
	ctx context.Context,
	config ast.MonitoringListCheckConfig,
) (bool, error) {
	// Get the object_id from the target table
	objectId, err := mlc.getTargetObjectId(ctx, config.PathToTarget)
	if err != nil {
		return false, errors.Wrap(err, "failed to get target object_id")
	}
	if objectId == "" {
		return false, nil
	}

	// Return false to not early return and run step 2
	if mlc.ReturnFakeValue {
		return false, nil
	}

	return mlc.checkObjectIdsHaveRiskTopics(ctx, config, config.TargetTableName, []string{objectId})
}

// getTargetObjectId retrieves the object_id from the target table by navigating through PathToTarget.
func (mlc MonitoringListCheck) getTargetObjectId(
	ctx context.Context,
	pathToTarget []string,
) (string, error) {
	// If path is empty, the target is the trigger table itself
	if len(pathToTarget) == 0 {
		objectIdValue, ok := mlc.ClientObject.Data["object_id"]
		if !ok {
			return "", nil
		}
		objectId, ok := objectIdValue.(string)
		if !ok {
			return "", errors.New("object_id is not a string")
		}
		return objectId, nil
	}

	// For dry run, validate the path exists in the data model and return a fake value
	if mlc.ReturnFakeValue {
		_, err := DryRunGetDbField(mlc.DataModel, mlc.ClientObject.TableName, pathToTarget, "object_id")
		if err != nil {
			return "", err
		}
		return "fake_object_id_for_dry_run", nil
	}

	// Use the IngestedDataReader to navigate to the target table and get the object_id
	db, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return "", errors.Wrap(err, "failed to create client db executor")
	}

	fieldValue, err := mlc.IngestedDataReader.GetDbField(
		ctx,
		db,
		models.DbFieldReadParams{
			TriggerTableName: mlc.ClientObject.TableName,
			Path:             pathToTarget,
			FieldName:        "object_id",
			DataModel:        mlc.DataModel,
			ClientObject:     mlc.ClientObject,
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to get object_id from target table")
	}
	if fieldValue == nil {
		return "", nil
	}

	objectId, ok := fieldValue.(string)
	if !ok {
		return "", errors.New("object_id is not a string")
	}

	return objectId, nil
}

// validateMonitoringListCheckConfig validates the MonitoringListCheckConfig and returns a slice of errors.
// It validates:
//   - targetTableName is not empty
//   - each linkedTableChecks entry has a non-empty tableName
//   - each linkedTableChecks entry has exactly one of linkToSingleName or navigationOption
//   - if navigationOption is present, targetTableName, targetFieldName, sourceTableName, and sourceFieldName are all non-empty
func (mlc MonitoringListCheck) validateMonitoringListCheckConfig(config ast.MonitoringListCheckConfig) []error {
	errs := make([]error, 0)

	// Validate target table name
	if config.TargetTableName == "" {
		errs = append(errs, errors.Join(
			ast.ErrArgumentRequired,
			ast.NewNamedArgumentError("targetTableName"),
			errors.New("targetTableName is required"),
		))
	}

	for i, check := range config.LinkedTableChecks {
		// Validate table name
		if check.TableName == "" {
			errs = append(errs, errors.Join(
				ast.ErrArgumentRequired,
				ast.NewNamedArgumentError(fmt.Sprintf("linkedTableChecks[%d].tableName", i)),
				errors.Newf("linkedTableChecks[%d].tableName is required", i),
			))
		}

		// Exactly one of linkToSingleName or navigationOption must be present
		hasLink := check.LinkToSingleName != nil
		hasNav := check.NavigationOption != nil

		if hasLink == hasNav {
			var err error
			if hasLink {
				err = errors.Join(
					ast.ErrArgumentInvalidType,
					ast.NewNamedArgumentError(fmt.Sprintf("linkedTableChecks[%d]", i)),
					errors.Newf("linkedTableChecks[%d]: cannot have both linkToSingleName and navigationOption", i),
				)
			} else {
				err = errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf("linkedTableChecks[%d]", i)),
					errors.Newf("linkedTableChecks[%d]: either linkToSingleName or navigationOption is required", i),
				)
			}
			errs = append(errs, err)
		}

		// Validate NavigationOption if present
		if hasNav && check.NavigationOption != nil {
			nav := check.NavigationOption
			if nav.TargetTableName == "" {
				errs = append(errs, errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf(
						"linkedTableChecks[%d].navigationOption.targetTableName", i)),
					errors.Newf("linkedTableChecks[%d].navigationOption.targetTableName is required", i),
				))
			}
			if nav.TargetFieldName == "" {
				errs = append(errs, errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf(
						"linkedTableChecks[%d].navigationOption.targetFieldName", i)),
					errors.Newf("linkedTableChecks[%d].navigationOption.targetFieldName is required", i),
				))
			}
			if nav.SourceTableName == "" {
				errs = append(errs, errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf(
						"linkedTableChecks[%d].navigationOption.sourceTableName", i)),
					errors.Newf("linkedTableChecks[%d].navigationOption.sourceTableName is required", i),
				))
			}
			if nav.SourceFieldName == "" {
				errs = append(errs, errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf(
						"linkedTableChecks[%d].navigationOption.sourceFieldName", i)),
					errors.Newf("linkedTableChecks[%d].navigationOption.sourceFieldName is required", i),
				))
			}
			if nav.OrderingFieldName == "" {
				errs = append(errs, errors.Join(
					ast.ErrArgumentRequired,
					ast.NewNamedArgumentError(fmt.Sprintf(
						"linkedTableChecks[%d].navigationOption.orderingFieldName", i)),
					errors.Newf("linkedTableChecks[%d].navigationOption.orderingFieldName is required", i),
				))
			}
		}
	}

	return errs
}

// checkLinkedTableViaLinkToSingle checks a linked table using LinkToSingle (single object).
func (mlc MonitoringListCheck) checkLinkedTableViaLinkToSingle(
	ctx context.Context,
	config ast.MonitoringListCheckConfig,
	linkedCheck ast.LinkedTableCheck,
) (bool, error) {
	// Build path: PathToTarget + LinkToSingleName
	path := append(config.PathToTarget, *linkedCheck.LinkToSingleName)

	// For dry run, validate the path exists
	if mlc.ReturnFakeValue {
		_, err := DryRunGetDbField(mlc.DataModel, mlc.ClientObject.TableName, path, "object_id")
		if err != nil {
			return false, err
		}
		return false, nil
	}

	// Get the object_id from the linked table
	db, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return false, errors.Wrap(err, "failed to create client db executor")
	}

	fieldValue, err := mlc.IngestedDataReader.GetDbField(ctx, db, models.DbFieldReadParams{
		TriggerTableName: mlc.ClientObject.TableName,
		Path:             path,
		FieldName:        "object_id",
		DataModel:        mlc.DataModel,
		ClientObject:     mlc.ClientObject,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to get object_id from linked table")
	}
	if fieldValue == nil {
		return false, nil
	}

	objectId, ok := fieldValue.(string)
	if !ok {
		return false, errors.New("object_id is not a string")
	}

	// Check if this object has risk topics
	return mlc.checkObjectIdsHaveRiskTopics(ctx, config, linkedCheck.TableName, []string{objectId})
}

// checkLinkedTableViaNavigation checks a linked table using NavigationOption (multiple objects).
func (mlc MonitoringListCheck) checkLinkedTableViaNavigation(
	ctx context.Context,
	config ast.MonitoringListCheckConfig,
	linkedCheck ast.LinkedTableCheck,
) (bool, error) {
	nav := linkedCheck.NavigationOption

	// For dry run, build data model with navigation options and validate the navigation exists
	if mlc.ReturnFakeValue {
		dataModelWithNav, err := mlc.buildDataModelWithNavigationOptions(ctx)
		if err != nil {
			return false, errors.Wrap(err, "failed to build data model with navigation options for dry run")
		}
		_, err = DryRunListIngestedObjects(dataModelWithNav, *nav, "object_id")
		if err != nil {
			return false, err
		}
		return false, nil
	}

	// Get the source field value to filter by
	sourceFieldValue, err := mlc.getSourceFieldValue(ctx, config, *nav)
	if err != nil {
		return false, errors.Wrap(err, "failed to get source field value")
	}
	if sourceFieldValue == nil {
		return false, nil
	}

	sourceFieldValueStr, ok := sourceFieldValue.(string)
	if !ok {
		return false, errors.New("source field value is not a string")
	}

	// Get target table from data model
	targetTable, ok := mlc.DataModel.Tables[nav.TargetTableName]
	if !ok {
		return false, errors.Newf("target table %s not found in data model", nav.TargetTableName)
	}

	// Create executor for client DB
	db, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return false, errors.Wrap(err, "failed to create client db executor")
	}

	// Fetch objects in batches using cursor pagination
	var cursorId *string
	for {
		// Request limit+1 to detect if there are more results
		objects, err := mlc.IngestedDataReader.ListIngestedObjects(
			ctx,
			db,
			targetTable,
			models.ExplorationOptions{
				SourceTableName:   nav.SourceTableName,
				FilterFieldName:   nav.TargetFieldName,
				FilterFieldValue:  models.NewStringOrNumberFromString(sourceFieldValueStr),
				OrderingFieldName: nav.OrderingFieldName,
			},
			cursorId,
			linkedTableCheckBatchSize+1,
			"object_id",
		)
		if err != nil {
			return false, errors.Wrap(err, "failed to list ingested objects")
		}

		if len(objects) == 0 {
			break
		}

		// Check if there are more results beyond this batch
		hasMore := len(objects) > linkedTableCheckBatchSize
		if hasMore {
			objects = objects[:linkedTableCheckBatchSize] // Trim to batch size
		}

		// Extract object_ids from the batch
		objectIds := make([]string, len(objects))
		for i, obj := range objects {
			objectIds[i] = obj.Data["object_id"].(string)
		}

		// Check if any of these objects have risk topics
		if len(objectIds) > 0 {
			hasRiskTopic, err := mlc.checkObjectIdsHaveRiskTopics(ctx, config, linkedCheck.TableName, objectIds)
			if err != nil {
				return false, err
			}
			if hasRiskTopic {
				return true, nil
			}
		}

		// If no more results, we're done
		if !hasMore {
			break
		}

		// Set cursor for next batch (use the last object's object_id)
		lastObjId, ok := objects[len(objects)-1].Data["object_id"].(string)
		if !ok {
			break
		}
		cursorId = &lastObjId
	}

	return false, nil
}

// getSourceFieldValue gets the value of the source field to use for filtering.
func (mlc MonitoringListCheck) getSourceFieldValue(
	ctx context.Context,
	config ast.MonitoringListCheckConfig,
	nav ast.NavigationOption,
) (any, error) {
	// If PathToTarget is empty, source is the trigger table itself
	if len(config.PathToTarget) == 0 {
		return mlc.ClientObject.Data[nav.SourceFieldName], nil
	}

	// Navigate to the source table and get the field value
	db, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client db executor")
	}

	return mlc.IngestedDataReader.GetDbField(ctx, db, models.DbFieldReadParams{
		TriggerTableName: mlc.ClientObject.TableName,
		Path:             config.PathToTarget,
		FieldName:        nav.SourceFieldName,
		DataModel:        mlc.DataModel,
		ClientObject:     mlc.ClientObject,
	})
}

// checkObjectIdsHaveRiskTopics checks if any of the given object_ids have matching risk topics.
func (mlc MonitoringListCheck) checkObjectIdsHaveRiskTopics(
	ctx context.Context,
	config ast.MonitoringListCheckConfig,
	objectType string,
	objectIds []string,
) (bool, error) {
	filter := models.ObjectRiskTopicsMetadataFilter{
		OrgId:      mlc.OrgId,
		ObjectType: objectType,
		ObjectIds:  objectIds,
	}

	topics := make([]models.RiskTopic, 0, len(config.TopicFilters))
	for _, t := range config.TopicFilters {
		topic := models.RiskTopicFrom(t)
		if topic != models.RiskTopicUnknown {
			topics = append(topics, topic)
		}
	}
	filter.Topics = topics

	exec := mlc.ExecutorFactory.NewExecutor()
	results, err := mlc.Repository.FindObjectRiskTopicsMetadata(ctx, exec, filter)
	if err != nil {
		return false, errors.Wrap(err, "failed to list object risk topics")
	}

	return len(results) > 0, nil
}

// buildDataModelWithNavigationOptions builds a data model with navigation options for dry run validation.
// This fetches pivots and indexes to compute navigation options, similar to DataModelUsecase.GetDataModel
// with IncludeNavigationOptions: true.
func (mlc MonitoringListCheck) buildDataModelWithNavigationOptions(ctx context.Context) (models.DataModel, error) {
	exec := mlc.ExecutorFactory.NewExecutor()

	// Fetch pivots
	pivotsMeta, err := mlc.Repository.ListPivots(ctx, exec, mlc.OrgId, nil, true)
	if err != nil {
		return models.DataModel{}, errors.Wrap(err, "failed to list pivots")
	}

	// Enrich pivots with data model
	pivots := make([]models.Pivot, len(pivotsMeta))
	for i, pivot := range pivotsMeta {
		pivots[i] = pivot.Enrich(mlc.DataModel)
	}

	// Fetch indexes from client db
	clientDb, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return models.DataModel{}, errors.Wrap(err, "failed to create client db executor")
	}

	indexes, err := mlc.ClientDbRepository.ListAllIndexes(ctx, clientDb, models.IndexTypeNavigation)
	if err != nil {
		return models.DataModel{}, errors.Wrap(err, "failed to list indexes")
	}

	// Add navigation options to data model
	return mlc.DataModel.AddNavigationOptionsToDataModel(indexes, pivots), nil
}
