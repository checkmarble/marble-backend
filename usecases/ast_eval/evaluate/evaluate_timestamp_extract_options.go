package evaluate

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type TimestampExtractOptionsEvaluator struct {
	executorFactory executor_factory.ExecutorFactory
	orgReader       orgReader
	orgId           uuid.UUID
}

func NewTimestampExtractOptionsEvaluator(
	executorFacotry executor_factory.ExecutorFactory,
	orgReaderRepository orgReader,
	orgId uuid.UUID,
) TimestampExtractOptionsEvaluator {
	return TimestampExtractOptionsEvaluator{
		executorFactory: executorFacotry,
		orgReader:       orgReaderRepository,
		orgId:           orgId,
	}
}

func (f TimestampExtractOptionsEvaluator) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	part, err := AdaptNamedArgument(arguments.NamedArgs, "part", adaptArgumentToString)
	if err != nil {
		return nil, []error{ast.NewNamedArgumentError("part")}
	}
	if !slices.Contains(validTimestampExtractParts, part) {
		return nil, []error{ast.NewNamedArgumentError("part")}
	}

	ranges, err := AdaptNamedArgument(arguments.NamedArgs, "ranges", adaptArgumentToListOfRanges)
	if err != nil {
		return nil, []error{ast.NewNamedArgumentError("ranges")}
	}

	organization, err := f.orgReader.GetOrganizationById(ctx, f.executorFactory.NewExecutor(), f.orgId)
	if err != nil {
		return nil, []error{errors.Wrap(err, "failed to read organization timezone")}
	}

	tz := "UTC"
	if organization.DefaultScenarioTimezone != nil {
		tz = *organization.DefaultScenarioTimezone
	}

	return ast.TimestampExtractOptions{
		Part:     part,
		Ranges:   ranges,
		Timezone: tz,
	}, nil
}
