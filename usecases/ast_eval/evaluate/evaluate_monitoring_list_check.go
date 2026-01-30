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

type MonitoringListCheckRepository interface {
	ListObjectRiskTopics(
		ctx context.Context,
		exec repositories.Executor,
		filter models.ObjectRiskTopicFilter,
		paginationAndSorting models.PaginationAndSorting,
	) ([]models.ObjectRiskTopic, error)
}

type MonitoringListCheck struct {
	ExecutorFactory executor_factory.ExecutorFactory

	OrgId              uuid.UUID
	ClientObject       models.ClientObject
	DataModel          models.DataModel
	Repository         MonitoringListCheckRepository
	IngestedDataReader repositories.IngestedDataReadRepository
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

	// Step 2: if Step 1 found anything, then use the LinkedTableChecks checks

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

	// For dry run, return true to indicate a potential match exists
	if mlc.ReturnFakeValue {
		return true, nil
	}

	// Build the filter for querying object_risk_topics
	filter := models.ObjectRiskTopicFilter{
		OrgId:      mlc.OrgId,
		ObjectType: &config.TargetTableName,
		ObjectIds:  []string{objectId},
	}

	// If topic filters are provided, filter by those topics
	// Otherwise, check for any topic (HasAnyTopic: true)
	if len(config.TopicFilters) > 0 {
		topics := make([]models.RiskTopic, 0, len(config.TopicFilters))
		for _, t := range config.TopicFilters {
			topic := models.RiskTopicFrom(t)
			if topic != models.RiskTopicUnknown {
				topics = append(topics, topic)
			}
		}
		filter.Topics = topics
	} else {
		filter.HasAnyTopic = true
	}

	// Query the object_risk_topics table
	exec := mlc.ExecutorFactory.NewExecutor()

	results, err := mlc.Repository.ListObjectRiskTopics(ctx, exec, filter, models.PaginationAndSorting{
		Limit:   1,
		Sorting: models.SortingFieldCreatedAt,
		Order:   models.SortingOrderDesc,
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to list object risk topics")
	}

	return len(results) > 0, nil
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

	fieldValue, err := mlc.IngestedDataReader.GetDbField(ctx, db, models.DbFieldReadParams{
		TriggerTableName: mlc.ClientObject.TableName,
		Path:             pathToTarget,
		FieldName:        "object_id",
		DataModel:        mlc.DataModel,
		ClientObject:     mlc.ClientObject,
	})
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
		}
	}

	return errs
}
