package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/google/uuid"
)

type CustomListUseCase struct {
	transactionFactory repositories.TransactionFactory
	CustomListRepository     repositories.CustomListRepository
}

func (usecase *CustomListUseCase) GetCustomLists(ctx context.Context, orgId string) ([]models.CustomList, error) {
	return usecase.CustomListRepository.AllCustomLists(nil, orgId)
}

func (usecase *CustomListUseCase) CreateCustomList(ctx context.Context, createCustomList models.CreateCustomListInput) error {
	newCustomListId := uuid.NewString()
	return usecase.CustomListRepository.CreateCustomList(nil, createCustomList, newCustomListId)
}

func (usecase *CustomListUseCase) UpdateCustomList(ctx context.Context, updateCustomList models.UpdateCustomListInput) (models.CustomList, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		if updateCustomList.Name != nil || updateCustomList.Description != nil {
			err := usecase.CustomListRepository.UpdateCustomList(tx, updateCustomList)
			if err != nil {
				return models.CustomList{}, err
			}
		} 
		return usecase.CustomListRepository.GetCustomListById(tx, updateCustomList.Id)
	})
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(ctx context.Context, deleteCustomList models.DeleteCustomListInput) error {
	return usecase.CustomListRepository.SoftDeleteCustomList(nil, deleteCustomList)
}

func (usecase *CustomListUseCase) GetCustomListById(ctx context.Context, id string) (models.CustomList, error) {
	return usecase.CustomListRepository.GetCustomListById(nil, id)
}

func (usecase *CustomListUseCase) GetCustomListValues(ctx context.Context, getCustomListValues models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	return usecase.CustomListRepository.GetCustomListValues(nil, getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(ctx context.Context, addCustomListValue models.AddCustomListValueInput) error {
	newCustomListValueId := uuid.NewString()
	return usecase.CustomListRepository.AddCustomListValue(nil, addCustomListValue, newCustomListValueId)
}

func (usecase *CustomListUseCase) DeleteCustomListValue(ctx context.Context, deleteCustomListValue models.DeleteCustomListValueInput) error {
	return usecase.CustomListRepository.DeleteCustomListValue(nil, deleteCustomListValue)
}