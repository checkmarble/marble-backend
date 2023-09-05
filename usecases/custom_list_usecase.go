package usecases

import (
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type CustomListUseCase struct {
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityCustomList
	transactionFactory      repositories.TransactionFactory
	CustomListRepository    repositories.CustomListRepository
}

func (usecase *CustomListUseCase) GetCustomLists() ([]models.CustomList, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return []models.CustomList{}, err
	}
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
	if err := usecase.enforceSecurity.CreateCustomList(); err != nil {
		return models.CustomList{}, err
	}

	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		newCustomListId := uuid.NewString()
		organizationId, err := usecase.organizationIdOfContext()
		if err != nil {
			return models.CustomList{}, err
		}

		err = usecase.CustomListRepository.CreateCustomList(tx, createCustomList, organizationId, newCustomListId)
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
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		if updateCustomList.Name != nil || updateCustomList.Description != nil {
			customList, err := usecase.CustomListRepository.GetCustomListById(tx, updateCustomList.Id)
			if err != nil {
				return models.CustomList{}, err
			}
			if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
				return models.CustomList{}, err
			}
			err = usecase.CustomListRepository.UpdateCustomList(tx, updateCustomList)
			if err != nil {
				return models.CustomList{}, err
			}
		}
		return usecase.CustomListRepository.GetCustomListById(tx, updateCustomList.Id)
	})
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(deleteCustomList models.DeleteCustomListInput) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(tx, deleteCustomList.Id)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.SoftDeleteCustomList(tx, deleteCustomList)
	})
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
	// CustomListValues doesn't know the OrganizationId of the list
	// so we call GetCustomListById so that it check if we are allowed to read it
	if _, err := usecase.GetCustomListById(getCustomListValues.Id); err != nil {
		return []models.CustomListValue{}, err
	}
	return usecase.CustomListRepository.GetCustomListValues(nil, getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(addCustomListValue models.AddCustomListValueInput) (models.CustomListValue, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomListValue, error) {
		customList, err := usecase.CustomListRepository.GetCustomListById(tx, addCustomListValue.CustomListId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return models.CustomListValue{}, err
		}
		newCustomListValueId := uuid.NewString()

		err = usecase.CustomListRepository.AddCustomListValue(tx, addCustomListValue, newCustomListValueId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		return usecase.CustomListRepository.GetCustomListValueById(tx, newCustomListValueId)
	})
}

func (usecase *CustomListUseCase) DeleteCustomListValue(deleteCustomListValue models.DeleteCustomListValueInput) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(tx, deleteCustomListValue.CustomListId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.DeleteCustomListValue(tx, deleteCustomListValue)
	})
}
