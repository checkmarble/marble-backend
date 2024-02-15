package usecases

import (
	"context"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
)

type CustomListUseCase struct {
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityCustomList
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	CustomListRepository    repositories.CustomListRepository
}

func (usecase *CustomListUseCase) GetCustomLists(ctx context.Context) ([]models.CustomList, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return []models.CustomList{}, err
	}
	customLists, err := usecase.CustomListRepository.AllCustomLists(ctx, usecase.executorFactory.NewExecutor(), organizationId)
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

	list, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (models.CustomList, error) {
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

	tracking.TrackEvent(ctx, models.AnalyticsListCreated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) UpdateCustomList(ctx context.Context, updateCustomList models.UpdateCustomListInput) (models.CustomList, error) {
	list, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (models.CustomList, error) {
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

	tracking.TrackEvent(ctx, models.AnalyticsListUpdated, map[string]interface{}{"list_id": list.Id})

	return list, nil
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(ctx context.Context, listId string) error {
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
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

	tracking.TrackEvent(ctx, models.AnalyticsListDeleted, map[string]interface{}{"list_id": listId})
	return nil
}

func (usecase *CustomListUseCase) GetCustomListById(ctx context.Context, id string) (models.CustomList, error) {
	customList, err := usecase.CustomListRepository.GetCustomListById(ctx, usecase.executorFactory.NewExecutor(), id)
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
	return usecase.CustomListRepository.GetCustomListValues(ctx, usecase.executorFactory.NewExecutor(), getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(ctx context.Context, addCustomListValue models.AddCustomListValueInput) (models.CustomListValue, error) {
	value, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (models.CustomListValue, error) {
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

	tracking.TrackEvent(ctx, models.AnalyticsListValueCreated, map[string]interface{}{"list_id": addCustomListValue.CustomListId})

	return value, nil
}

func (usecase *CustomListUseCase) DeleteCustomListValue(ctx context.Context, deleteCustomListValue models.DeleteCustomListValueInput) error {
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
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

	tracking.TrackEvent(ctx, models.AnalyticsListValueDeleted, map[string]interface{}{"list_id": deleteCustomListValue.CustomListId})

	return nil
}
