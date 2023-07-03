package evaluate

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
)

type CustomListValuesAccess struct {
	CustomListRepository repositories.CustomListRepository
	Creds                models.Credentials
}

func NewCustomListValuesAccess(clr repositories.CustomListRepository, creds models.Credentials) CustomListValuesAccess {
	return CustomListValuesAccess{
		CustomListRepository: clr,
		Creds:                creds,
	}
}

func (clva CustomListValuesAccess) Evaluate(arguments ast.Arguments) (any, error) {
	listId, ok := arguments.NamedArgs["customListId"].((string))
	if !ok {
		return nil, fmt.Errorf("customListId is not a string %w", ErrRuntimeExpression)
	}

	list, err := clva.CustomListRepository.GetCustomListById(nil, listId)
	if err != nil {
		return nil, errors.New("list not found")
	}
	if err := utils.EnforceOrganizationAccess(clva.Creds, list.OrgId); err != nil {
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
