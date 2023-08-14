package evaluate

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
	"marble/marble-backend/utils"
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

func (clva CustomListValuesAccess) Evaluate(arguments ast.Arguments) (any, error) {
	listId, ok := arguments.NamedArgs["customListId"].((string))
	if !ok {
		return nil, fmt.Errorf("customListId is not a string %w", models.ErrRuntimeExpression)
	}

	list, err := clva.CustomListRepository.GetCustomListById(nil, listId)
	if err != nil {
		return nil, errors.New("list not found")
	}
	if err := clva.EnforceSecurity.ReadOrganization(list.OrgId); err != nil {
		return nil, err
	}

	listValues, err := clva.CustomListRepository.GetCustomListValues(nil, models.GetCustomListValuesInput{
		Id: listId,
	})
	if err != nil {
		return nil, err
	}

	return utils.Map(
		listValues,
		func(v models.CustomListValue) string { return v.Value },
	), nil
}
