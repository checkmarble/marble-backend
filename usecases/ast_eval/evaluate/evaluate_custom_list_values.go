package evaluate

import (
	"errors"

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

func (clva CustomListValuesAccess) Evaluate(arguments ast.Arguments) (any, []error) {
	listId, err := AdaptNamedArgument(arguments.NamedArgs, "customListId", adaptArgumentToString)

	if err != nil {
		return MakeEvaluateError(err)
	}

	list, err := clva.CustomListRepository.GetCustomListById(nil, listId)
	if err != nil {
		return MakeEvaluateError(errors.New("list not found"))
	}
	if err := clva.EnforceSecurity.ReadOrganization(list.OrganizationId); err != nil {
		return MakeEvaluateError(err)
	}

	listValues, err := clva.CustomListRepository.GetCustomListValues(nil, models.GetCustomListValuesInput{
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
