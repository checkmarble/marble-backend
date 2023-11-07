package usecases

import (
	"context"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type CustomListUseCase struct {
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityCustomList
	transactionFactory      transaction.TransactionFactory
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

func (usecase *CustomListUseCase) CreateCustomList(ctx context.Context, createCustomList models.CreateCustomListInput) (models.CustomList, error) {
	if err := usecase.enforceSecurity.CreateCustomList(); err != nil {
		return models.CustomList{}, err
	}

	list, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		newCustomListId := uuid.NewString()
		organizationId, err := usecase.organizationIdOfContext()
		if err != nil {
			return models.CustomList{}, err
		}

		err = usecase.CustomListRepository.CreateCustomList(tx, createCustomList, organizationId, newCustomListId)
		if repositories.IsUniqueViolationError(err) {
			return models.CustomList{}, models.DuplicateValueError
		}
		if err != nil {
			return models.CustomList{}, err
		}
		return usecase.CustomListRepository.GetCustomListById(tx, newCustomListId)
	})
	if err != nil {
		return models.CustomList{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListCreated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) UpdateCustomList(ctx context.Context, updateCustomList models.UpdateCustomListInput) (models.CustomList, error) {
	list, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
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
	if err != nil {
		return models.CustomList{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListUpdated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(ctx context.Context, listId string) error {
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(tx, listId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.SoftDeleteCustomList(tx, listId)
	})
	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListDeleted, map[string]interface{}{"list_id": listId})
	return nil
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

func (usecase *CustomListUseCase) AddCustomListValue(ctx context.Context, addCustomListValue models.AddCustomListValueInput) (models.CustomListValue, error) {
	value, err := transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomListValue, error) {
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
	if err != nil {
		return models.CustomListValue{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListValueCreated, map[string]interface{}{"list_id": addCustomListValue.CustomListId})

	return value, nil
}

func (usecase *CustomListUseCase) DeleteCustomListValue(ctx context.Context, deleteCustomListValue models.DeleteCustomListValueInput) error {
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(tx, deleteCustomListValue.CustomListId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.DeleteCustomListValue(tx, deleteCustomListValue)
	})

	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListValueDeleted, map[string]interface{}{"list_id": deleteCustomListValue.CustomListId})

	return nil
}
