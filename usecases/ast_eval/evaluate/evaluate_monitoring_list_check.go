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

const (
	linkedTableCheckBatchSize      = 100
	maxConcurrentLinkedTableChecks = 10
)

type MonitoringListCheckRepository interface {
	FindEntityAnnotationsWithRiskTags(
		ctx context.Context,
		exec repositories.Executor,
		filter models.EntityAnnotationRiskTagsFilter,
	) ([]models.EntityAnnotation, error)
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

	// Create executors once for all operations
	exec := mlc.ExecutorFactory.NewExecutor()
	execDbClient, err := mlc.ExecutorFactory.NewClientDbExecutor(ctx, mlc.OrgId)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "failed to create client db executor"))
	}

	// Step 1: fetch the ingested data based on config, get the `object_id` and query in entity_annotations table if the element has a risk tag assigned
	hasRiskTag, err := mlc.checkTargetObjectHasRiskTag(ctx, exec, execDbClient, config)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "failed to check target object risk tags"))
	}
	if hasRiskTag {
		return true, nil
	}

	// Step 2: check LinkedTableChecks for risk topics in parallel
	if len(config.LinkedTableChecks) == 0 {
		return false, nil
	}

	// Create cancellable context and channels for early exit
	checkCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type checkResult struct {
		hasRiskTag bool
		err        error
	}
	results := make(chan checkResult, len(config.LinkedTableChecks))

	// Limit concurrent goroutines to avoid overwhelming connection pool
	sem := make(chan struct{}, maxConcurrentLinkedTableChecks)

	// Launch goroutines
	for _, linkedCheck := range config.LinkedTableChecks {
		go func(linkedCheck ast.LinkedTableCheck) {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }() // Release semaphore
			case <-checkCtx.Done():
				results <- checkResult{err: checkCtx.Err()}
				return
			}

			var hasRisk bool
			var err error

			if linkedCheck.LinkToSingleName != nil {
				hasRisk, err = mlc.checkLinkedTableViaLinkToSingle(checkCtx, exec, execDbClient, config, linkedCheck)
			} else if linkedCheck.NavigationOption != nil {
				hasRisk, err = mlc.checkLinkedTableViaNavigation(checkCtx, exec, execDbClient, config, linkedCheck)
			}

			if err != nil {
				err = errors.Wrapf(err, "failed to check linked table %s", linkedCheck.TableName)
			}

			results <- checkResult{hasRiskTag: hasRisk, err: err}
		}(linkedCheck)
	}

	// Collect results - return immediately on first match
	completed := 0
	for completed < len(config.LinkedTableChecks) {
		select {
		case result := <-results:
			completed++

			if result.err != nil && !errors.Is(result.err, context.Canceled) {
				cancel() // Cancel remaining goroutines
				return MakeEvaluateError(result.err)
			}

			if result.hasRiskTag {
				cancel()         // Cancel remaining goroutines
				return true, nil // Return immediately without waiting
			}

		case <-ctx.Done():
			cancel()
			return false, nil
		}
	}

	return false, nil
}

// checkTargetObjectHasRiskTag checks if the target object has any matching risk tags.
// It fetches the object_id from the target table using PathToTarget, then queries entity_annotations.
func (mlc MonitoringListCheck) checkTargetObjectHasRiskTag(
	ctx context.Context,
	exec repositories.Executor,
	execDbClient repositories.Executor,
	config ast.MonitoringListCheckConfig,
) (bool, error) {
	// Get the object_id from the target table
	objectId, err := mlc.getTargetObjectId(ctx, execDbClient, config.PathToTarget)
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

	return mlc.checkObjectIdsHaveRiskTags(ctx, exec, config, config.TargetTableName, []string{objectId})
}

// getTargetObjectId retrieves the object_id from the target table by navigating through PathToTarget.
func (mlc MonitoringListCheck) getTargetObjectId(
	ctx context.Context,
	execDbClient repositories.Executor,
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
	fieldValue, err := mlc.IngestedDataReader.GetDbField(
		ctx,
		execDbClient,
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
	exec repositories.Executor,
	execDbClient repositories.Executor,
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
	fieldValue, err := mlc.IngestedDataReader.GetDbField(ctx, execDbClient, models.DbFieldReadParams{
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

	// Check if this object has risk tags
	return mlc.checkObjectIdsHaveRiskTags(ctx, exec, config, linkedCheck.TableName, []string{objectId})
}

// checkLinkedTableViaNavigation checks a linked table using NavigationOption (multiple objects).
func (mlc MonitoringListCheck) checkLinkedTableViaNavigation(
	ctx context.Context,
	exec repositories.Executor,
	execDbClient repositories.Executor,
	config ast.MonitoringListCheckConfig,
	linkedCheck ast.LinkedTableCheck,
) (bool, error) {
	nav := linkedCheck.NavigationOption

	// For dry run, build data model with navigation options and validate the navigation exists
	if mlc.ReturnFakeValue {
		dataModelWithNav, err := mlc.buildDataModelWithNavigationOptions(ctx, exec, execDbClient)
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
	sourceFieldValue, err := mlc.getSourceFieldValue(ctx, execDbClient, config, *nav)
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

	// Only fetch at most linkedCheckBatchSize objects, in case of more objects we can miss the risk topic
	// but we want to avoid fetching too many objects for performance reasons.
	objects, err := mlc.IngestedDataReader.ListIngestedObjects(
		ctx,
		execDbClient,
		targetTable,
		models.ExplorationOptions{
			SourceTableName:   nav.SourceTableName,
			FilterFieldName:   nav.TargetFieldName,
			FilterFieldValue:  models.NewStringOrNumberFromString(sourceFieldValueStr),
			OrderingFieldName: nav.OrderingFieldName,
		},
		nil,
		linkedTableCheckBatchSize,
		"object_id",
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to list ingested objects")
	}

	if len(objects) == 0 {
		return false, nil
	}

	// Extract object_ids from the batch
	objectIds := make([]string, len(objects))
	for i, obj := range objects {
		objectIds[i] = obj.Data["object_id"].(string)
	}

	// Check if any of these objects have risk tags
	if len(objectIds) > 0 {
		hasRiskTag, err := mlc.checkObjectIdsHaveRiskTags(ctx, exec, config, linkedCheck.TableName, objectIds)
		if err != nil {
			return false, err
		}
		if hasRiskTag {
			return true, nil
		}
	}

	return false, nil
}

// getSourceFieldValue gets the value of the source field to use for filtering.
func (mlc MonitoringListCheck) getSourceFieldValue(
	ctx context.Context,
	execDbClient repositories.Executor,
	config ast.MonitoringListCheckConfig,
	nav ast.NavigationOption,
) (any, error) {
	// If PathToTarget is empty, source is the trigger table itself
	if len(config.PathToTarget) == 0 {
		return mlc.ClientObject.Data[nav.SourceFieldName], nil
	}

	// Navigate to the source table and get the field value
	return mlc.IngestedDataReader.GetDbField(ctx, execDbClient, models.DbFieldReadParams{
		TriggerTableName: mlc.ClientObject.TableName,
		Path:             config.PathToTarget,
		FieldName:        nav.SourceFieldName,
		DataModel:        mlc.DataModel,
		ClientObject:     mlc.ClientObject,
	})
}

// checkObjectIdsHaveRiskTags checks if any of the given object_ids have matching risk tags.
func (mlc MonitoringListCheck) checkObjectIdsHaveRiskTags(
	ctx context.Context,
	exec repositories.Executor,
	config ast.MonitoringListCheckConfig,
	objectType string,
	objectIds []string,
) (bool, error) {
	filter := models.EntityAnnotationRiskTagsFilter{
		OrgId:      mlc.OrgId,
		ObjectType: objectType,
		ObjectIds:  objectIds,
	}

	tags := make([]models.RiskTag, 0, len(config.TopicFilters))
	for _, t := range config.TopicFilters {
		tag := models.RiskTagFrom(t)
		if tag != models.RiskTagUnknown {
			tags = append(tags, tag)
		}
	}
	filter.Tags = tags

	results, err := mlc.Repository.FindEntityAnnotationsWithRiskTags(ctx, exec, filter)
	if err != nil {
		return false, errors.Wrap(err, "failed to find risk tag annotations")
	}

	return len(results) > 0, nil
}

// buildDataModelWithNavigationOptions builds a data model with navigation options for dry run validation.
// This fetches pivots and indexes to compute navigation options, similar to DataModelUsecase.GetDataModel
// with IncludeNavigationOptions: true.
func (mlc MonitoringListCheck) buildDataModelWithNavigationOptions(
	ctx context.Context,
	exec repositories.Executor,
	execDbClient repositories.Executor,
) (models.DataModel, error) {
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
	indexes, err := mlc.ClientDbRepository.ListAllIndexes(ctx, execDbClient, models.IndexTypeNavigation)
	if err != nil {
		return models.DataModel{}, errors.Wrap(err, "failed to list indexes")
	}

	// Add navigation options to data model
	return mlc.DataModel.AddNavigationOptionsToDataModel(indexes, pivots), nil
}
