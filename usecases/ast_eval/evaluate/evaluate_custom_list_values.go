package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type CustomListValuesAccess struct {
	CustomListRepository repositories.CustomListRepository
	EnforceSecurity      security.EnforceSecurity
}

func NewCustomListValuesAccess(clr repositories.CustomListRepository, enforceSecurity security.EnforceSecurity) CustomListValuesAccess {
	return CustomListValuesAccess{
		CustomListRepository: clr,
		EnforceSecurity:      enforceSecurity,
	}
}

func (clva CustomListValuesAccess) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	listId, err := AdaptNamedArgument(arguments.NamedArgs, "customListId", adaptArgumentToString)

	if err != nil {
		return MakeEvaluateError(err)
	}

	list, err := clva.CustomListRepository.GetCustomListById(ctx, nil, listId)
	if err != nil {
		return MakeEvaluateError(ast.ErrListNotFound)
	}
	if err := clva.EnforceSecurity.ReadOrganization(list.OrganizationId); err != nil {
		return MakeEvaluateError(err)
	}

	listValues, err := clva.CustomListRepository.GetCustomListValues(ctx, nil, models.GetCustomListValuesInput{
		Id: listId,
	})
	if err != nil {
		return MakeEvaluateError(err)
	}

	return utils.Map(
		listValues,
		func(v models.CustomListValue) string { return v.Value },
	), nil
}
