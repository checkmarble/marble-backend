package evaluate

import (
	"context"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
)

type orgReader interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type TimestampExtract struct {
	executorFactory   executor_factory.ExecutorFactory
	orgReadRepository orgReader
	organizationId    string
}

func NewTimestampExtract(
	executorFactory executor_factory.ExecutorFactory, orgReadRepository orgReader, organizationId string,
) TimestampExtract {
	return TimestampExtract{
		executorFactory:   executorFactory,
		orgReadRepository: orgReadRepository,
		organizationId:    organizationId,
	}
}

var validTimestampExtractParts = []string{
	"year",
	"month",
	"day_of_month",
	"day_of_week",
	"hour",
}

func (f TimestampExtract) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if val, ok := arguments.NamedArgs["timestamp"]; ok && val == nil {
		return nil, nil
	}

	var errs []error
	timestamp, timeErr := AdaptNamedArgument(arguments.NamedArgs, "timestamp", adaptArgumentToTime)
	errs = append(errs, timeErr)
	part, partErr := AdaptNamedArgument(arguments.NamedArgs, "part", adaptArgumentToString)
	errs = append(errs, partErr)
	if partErr == nil && !slices.Contains(validTimestampExtractParts, part) {
		errs = append(errs, ast.NewNamedArgumentError("part"))
	}

	errs = filterNilErrors(errs...)
	if len(errs) > 0 {
		return nil, errs
	}

	organization, err := f.orgReadRepository.GetOrganizationById(ctx,
		f.executorFactory.NewExecutor(), f.organizationId)
	if err != nil {
		return nil, []error{errors.Wrap(err, "failed to get organization")}
	}
	var timezone string
	if organization.DefaultScenarioTimezone != nil {
		timezone = *organization.DefaultScenarioTimezone
	} else {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, []error{errors.Wrap(err, "failed to load organization timezone")}
	}
	timestamp = timestamp.In(location)

	switch part {
	// for convenience, make sure it always returns an int (not a type wrapping an int like type.Month)
	case "year":
		return timestamp.Year(), nil
	case "month":
		return int(timestamp.Month()), nil
	case "day_of_month":
		return timestamp.Day(), nil
	case "day_of_week":
		weekday := int(timestamp.Weekday())
		if weekday == 0 {
			// Sunday is 0 in Go, but we want it to be 7
			weekday = 7
		}
		return weekday, nil
	case "hour":
		return timestamp.Hour(), nil
	default:
		// should not happen as per validation above. Return an error as a guard against future changes
		return nil, []error{errors.New("should not happen")}
	}
}
