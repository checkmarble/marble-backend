package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"

	"github.com/google/uuid"
)

type CustomListUseCase struct {
	enforceSecurity      security.EnforceSecurityCustomList
	transactionFactory   repositories.TransactionFactory
	CustomListRepository repositories.CustomListRepository
}

func (usecase *CustomListUseCase) GetCustomLists(organizationId string) ([]models.CustomList, error) {
	customLists, err := usecase.CustomListRepository.AllCustomLists(nil, organizationId)
	if err != nil {
		return []models.CustomList{}, err
	}
	for _, ci := range customLists {
		if err := usecase.enforceSecurity.ReadCustomList(ci); err != nil {
			return []models.CustomList{}, err
		}
	}
	return customLists, nil
}

func (usecase *CustomListUseCase) CreateCustomList(createCustomList models.CreateCustomListInput) (models.CustomList, error) {
	if err := usecase.enforceSecurity.CreateCustomList(createCustomList.OrganizationId); err != nil {
		return models.CustomList{}, err
	}
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		newCustomListId := uuid.NewString()
		err := usecase.CustomListRepository.CreateCustomList(tx, createCustomList, newCustomListId)
		if err != nil {
			if repositories.IsIsUniqueViolationError(err) {
				return models.CustomList{}, models.DuplicateValueError
			}
			return models.CustomList{}, err
		}
		return usecase.CustomListRepository.GetCustomListById(tx, newCustomListId)
	})
}

func (usecase *CustomListUseCase) UpdateCustomList(updateCustomList models.UpdateCustomListInput) (models.CustomList, error) {
	if err := usecase.enforceSecurity.CreateCustomList(updateCustomList.OrganizationId); err != nil {
		return models.CustomList{}, err
	}
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

func (usecase *CustomListUseCase) SoftDeleteCustomList(deleteCustomList models.DeleteCustomListInput) error {
	if err := usecase.enforceSecurity.CreateCustomList(deleteCustomList.OrganizationId); err != nil {
		return err
	}
	return usecase.CustomListRepository.SoftDeleteCustomList(nil, deleteCustomList)
}

func (usecase *CustomListUseCase) GetCustomListById(id string) (models.CustomList, error) {
	customList, err := usecase.CustomListRepository.GetCustomListById(nil, id)
	if err != nil {
		return models.CustomList{}, err
	}
	if err := usecase.enforceSecurity.ReadCustomList(customList); err != nil {
		return models.CustomList{}, err
	}
	return customList, nil
}

func (usecase *CustomListUseCase) GetCustomListValues(getCustomListValues models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	if _, err := usecase.GetCustomListById(getCustomListValues.Id); err != nil {
		return []models.CustomListValue{}, err
	}
	return usecase.CustomListRepository.GetCustomListValues(nil, getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(addCustomListValue models.AddCustomListValueInput) (models.CustomListValue, error) {
	if err := usecase.enforceSecurity.CreateCustomList(addCustomListValue.OrganizationId); err != nil {
		return models.CustomListValue{}, err
	}
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomListValue, error) {
		newCustomListValueId := uuid.NewString()
		err := usecase.CustomListRepository.AddCustomListValue(tx, addCustomListValue, newCustomListValueId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		return usecase.CustomListRepository.GetCustomListValueById(tx, newCustomListValueId)
	})
}

func (usecase *CustomListUseCase) DeleteCustomListValue(deleteCustomListValue models.DeleteCustomListValueInput) error {
	if err := usecase.enforceSecurity.CreateCustomList(deleteCustomListValue.OrganizationId); err != nil {
		return err
	}
	return usecase.CustomListRepository.DeleteCustomListValue(nil, deleteCustomListValue)
}
