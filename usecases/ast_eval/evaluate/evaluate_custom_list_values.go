package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type CustomListValuesAccess struct {
	CustomListRepository repositories.CustomListRepository
	EnforceSecurity      security.EnforceSecurity
	executorFactory      executor_factory.ExecutorFactory
}

func NewCustomListValuesAccess(
	clr repositories.CustomListRepository,
	enforceSecurity security.EnforceSecurity,
	executorFactory executor_factory.ExecutorFactory,
) CustomListValuesAccess {
	return CustomListValuesAccess{
		CustomListRepository: clr,
		EnforceSecurity:      enforceSecurity,
		executorFactory:      executorFactory,
	}
}

func (clva CustomListValuesAccess) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	exec := clva.executorFactory.NewExecutor()
	listId, err := AdaptNamedArgument(arguments.NamedArgs, "customListId", adaptArgumentToString)
	if err != nil {
		return MakeEvaluateError(err)
	}

	list, err := clva.CustomListRepository.GetCustomListById(ctx, exec, listId, true)
	if errors.Is(err, models.NotFoundError) {
		return MakeEvaluateError(ast.ErrListNotFound)
	} else if err != nil {
		return MakeEvaluateError(errors.Wrap(
			err,
			fmt.Sprintf("Error reading list %s", listId)))
	}
	if err := clva.EnforceSecurity.ReadOrganization(list.OrganizationId); err != nil {
		return MakeEvaluateError(errors.Wrap(err,
			fmt.Sprintf("Organization in credentials is not allowed to read this list %s", list.Id)))
	}

	listValues, err := clva.CustomListRepository.GetCustomListValues(ctx, exec, models.GetCustomListValuesInput{
		Id: listId,
	})
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err,
			fmt.Sprintf("Error reading values for list %s", list.Id)))
	}

	var valueFromListFn func(v models.CustomListValue) any

	switch list.Kind {
	case models.CustomListText:
		valueFromListFn = func(v models.CustomListValue) any { return pure_utils.Normalize(*v.Value) }
	case models.CustomListCidrs:
		valueFromListFn = func(v models.CustomListValue) any { return *v.CidrValue }
	}

	return pure_utils.Map(
		listValues,
		valueFromListFn,
	), nil
}
