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

func (usecase *CustomListUseCase) GetCustomLists(ctx context.Context) ([]models.CustomList, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return []models.CustomList{}, err
	}
	customLists, err := usecase.CustomListRepository.AllCustomLists(ctx, nil, organizationId)
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

	list, err := transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		newCustomListId := uuid.NewString()
		organizationId, err := usecase.organizationIdOfContext()
		if err != nil {
			return models.CustomList{}, err
		}

		err = usecase.CustomListRepository.CreateCustomList(ctx, tx, createCustomList, organizationId, newCustomListId)
		if repositories.IsUniqueViolationError(err) {
			return models.CustomList{}, models.DuplicateValueError
		}
		if err != nil {
			return models.CustomList{}, err
		}
		return usecase.CustomListRepository.GetCustomListById(ctx, tx, newCustomListId)
	})
	if err != nil {
		return models.CustomList{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListCreated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) UpdateCustomList(ctx context.Context, updateCustomList models.UpdateCustomListInput) (models.CustomList, error) {
	list, err := transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomList, error) {
		if updateCustomList.Name != nil || updateCustomList.Description != nil {
			customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, updateCustomList.Id)
			if err != nil {
				return models.CustomList{}, err
			}
			if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
				return models.CustomList{}, err
			}
			err = usecase.CustomListRepository.UpdateCustomList(ctx, tx, updateCustomList)
			if err != nil {
				return models.CustomList{}, err
			}
		}
		return usecase.CustomListRepository.GetCustomListById(ctx, tx, updateCustomList.Id)
	})
	if err != nil {
		return models.CustomList{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListUpdated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(ctx context.Context, listId string) error {
	err := usecase.transactionFactory.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, listId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.SoftDeleteCustomList(ctx, tx, listId)
	})
	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListDeleted, map[string]interface{}{"list_id": listId})
	return nil
}

func (usecase *CustomListUseCase) GetCustomListById(ctx context.Context, id string) (models.CustomList, error) {
	customList, err := usecase.CustomListRepository.GetCustomListById(ctx, nil, id)
	if err != nil {
		return models.CustomList{}, err
	}
	if err := usecase.enforceSecurity.ReadCustomList(customList); err != nil {
		return models.CustomList{}, err
	}
	return customList, nil
}

func (usecase *CustomListUseCase) GetCustomListValues(ctx context.Context, getCustomListValues models.GetCustomListValuesInput) ([]models.CustomListValue, error) {
	// CustomListValues doesn't know the OrganizationId of the list
	// so we call GetCustomListById so that it check if we are allowed to read it
	if _, err := usecase.GetCustomListById(ctx, getCustomListValues.Id); err != nil {
		return []models.CustomListValue{}, err
	}
	return usecase.CustomListRepository.GetCustomListValues(ctx, nil, getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(ctx context.Context, addCustomListValue models.AddCustomListValueInput) (models.CustomListValue, error) {
	value, err := transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.CustomListValue, error) {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, addCustomListValue.CustomListId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return models.CustomListValue{}, err
		}
		newCustomListValueId := uuid.NewString()

		err = usecase.CustomListRepository.AddCustomListValue(ctx, tx, addCustomListValue, newCustomListValueId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		return usecase.CustomListRepository.GetCustomListValueById(ctx, tx, newCustomListValueId)
	})
	if err != nil {
		return models.CustomListValue{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListValueCreated, map[string]interface{}{"list_id": addCustomListValue.CustomListId})

	return value, nil
}

func (usecase *CustomListUseCase) DeleteCustomListValue(ctx context.Context, deleteCustomListValue models.DeleteCustomListValueInput) error {
	err := usecase.transactionFactory.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, deleteCustomListValue.CustomListId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.DeleteCustomListValue(ctx, tx, deleteCustomListValue)
	})

	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsListValueDeleted, map[string]interface{}{"list_id": deleteCustomListValue.CustomListId})

	return nil
}
