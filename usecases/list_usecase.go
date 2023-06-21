package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/google/uuid"
)

type ListUseCase struct {
	transactionFactory repositories.TransactionFactory
	listRepository     repositories.ListRepository
}

func (usecase *ListUseCase) GetLists(ctx context.Context, orgId string) ([]models.List, error) {
	return usecase.listRepository.AllLists(nil, orgId)
}

func (usecase *ListUseCase) CreateList(ctx context.Context, createList models.CreateListInput) error {
	newListId := uuid.NewString()
	return usecase.listRepository.CreateList(nil, createList, newListId)
}

func (usecase *ListUseCase) UpdateList(ctx context.Context, updateList models.UpdateListInput) (models.List, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.List, error) {
		err := usecase.listRepository.UpdateList(tx, updateList)
		if err != nil {
			return models.List{}, err
		}
		return usecase.listRepository.GetListById(tx, models.GetListInput{
			Id:    updateList.Id,
			OrgId: updateList.OrgId,
		})
	})
}

func (usecase *ListUseCase) DeleteList(ctx context.Context, deleteList models.DeleteListInput) error {
	return usecase.listRepository.DeleteList(nil, deleteList)
}

func (usecase *ListUseCase) GetListValues(ctx context.Context, getListValues models.GetListValuesInput) ([]models.ListValue, error) {
	return usecase.listRepository.GetListValues(nil, getListValues)
}

func (usecase *ListUseCase) AddListValue(ctx context.Context, addListValue models.AddListValueInput) error {
	newListValueId := uuid.NewString()
	return usecase.listRepository.AddListValue(nil, addListValue, newListValueId)
}

func (usecase *ListUseCase) DeleteListValue(ctx context.Context, deleteListValue models.DeleteListValueInput) error {
	return usecase.listRepository.DeleteListValue(nil, deleteListValue)
}