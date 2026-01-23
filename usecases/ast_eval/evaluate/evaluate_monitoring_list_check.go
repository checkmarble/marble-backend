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

type MonitoringListCheckRepository interface{}

type MonitoringListCheck struct {
	ExecutorFactory executor_factory.ExecutorFactory

	OrgId              uuid.UUID
	ClientObject       models.ClientObject
	DataModel          models.DataModel
	Repository         MonitoringListCheckRepository
	IngestedDataReader repositories.IngestedDataReadRepository
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
	if errs := validateMonitoringListCheckConfig(config); len(errs) > 0 {
		return nil, errs
	}

	hasMatch := false

	return hasMatch, nil
}

func validateMonitoringListCheckConfig(config ast.MonitoringListCheckConfig) []error {
	errs := make([]error, 0)

	// Validate target table name
	if config.TargetTableName == "" {
		errs = append(errs, errors.Join(
			ast.ErrArgumentRequired,
			ast.NewNamedArgumentError("targetTableName"),
			errors.New("targetTableName is required"),
		))
	}

	// Validate linked table checks
	if len(config.LinkedTableChecks) == 0 {
		errs = append(errs, errors.Join(
			ast.ErrArgumentRequired,
			ast.NewNamedArgumentError("linkedTableChecks"),
			errors.New("at least one linkedTableCheck is required"),
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

		if !hasLink && !hasNav {
			errs = append(errs, errors.Join(
				ast.ErrArgumentRequired,
				ast.NewNamedArgumentError(fmt.Sprintf("linkedTableChecks[%d]", i)),
				errors.Newf("linkedTableChecks[%d]: either linkToSingleName or navigationOption is required", i),
			))
		}

		if hasLink && hasNav {
			errs = append(errs, errors.Join(
				ast.ErrArgumentInvalidType,
				ast.NewNamedArgumentError(fmt.Sprintf("linkedTableChecks[%d]", i)),
				errors.Newf("linkedTableChecks[%d]: cannot have both linkToSingleName and navigationOption", i),
			))
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
