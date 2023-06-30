package accessor

import (
	"marble/marble-backend/models/ast"
)

type CustomListAccessor struct {
	// customListUsecase *usecases.CustomListUseCase
}

func NewCustomListAccessor() CustomListAccessor {
	return CustomListAccessor{
		// customListUsecase: clu,
	}
}

func (cla CustomListAccessor) RetriveData(arg ast.Arguments) (any, error) {
	// listValues, err := cla.customListUsecase.GetCustomListValues(nil, models.GetCustomListValuesInput{
	// 	Id:    arg.Args[0].((string)),
	// 	OrgId: arg.Args[1].(string),
	// })
	// if err != nil {
	// 	return nil, err
	// }
	// for _, v := range listValues {
	// 	stringListValues = append(stringListValues, v.Value)
	// }
	return nil, nil
}
