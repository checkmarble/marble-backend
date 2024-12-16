package usecases

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
)

type CustomListUseCase struct {
	enforceSecurity      security.EnforceSecurityCustomList
	transactionFactory   executor_factory.TransactionFactory
	executorFactory      executor_factory.ExecutorFactory
	CustomListRepository repositories.CustomListRepository
}

func (usecase *CustomListUseCase) GetCustomLists(ctx context.Context, organizationId string) ([]models.CustomList, error) {
	customLists, err := usecase.CustomListRepository.AllCustomLists(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
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

func (usecase *CustomListUseCase) CreateCustomList(
	ctx context.Context,

	createCustomList models.CreateCustomListInput,
) (models.CustomList, error) {
	if err := usecase.enforceSecurity.CreateCustomList(); err != nil {
		return models.CustomList{}, err
	}

	list, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.CustomList, error) {
		newCustomListId := uuid.NewString()

		err := usecase.CustomListRepository.CreateCustomList(ctx, tx, createCustomList, newCustomListId)
		if repositories.IsUniqueViolationError(err) {
			return models.CustomList{}, models.ConflictError
		}
		if err != nil {
			return models.CustomList{}, err
		}
		return usecase.CustomListRepository.GetCustomListById(ctx, tx, newCustomListId)
	})
	if err != nil {
		return models.CustomList{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsListCreated, map[string]interface{}{
		"list_id": list.Id,
	})

	return list, nil
}

func (usecase *CustomListUseCase) UpdateCustomList(ctx context.Context,
	updateCustomList models.UpdateCustomListInput,
) (models.CustomList, error) {
	list, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.CustomList, error) {
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

	tracking.TrackEvent(ctx, models.AnalyticsListUpdated, map[string]interface{}{
		"list_id": list.Id,
	})

	return list, nil
}

func (usecase *CustomListUseCase) SoftDeleteCustomList(ctx context.Context, listId string) error {
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
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

	tracking.TrackEvent(ctx, models.AnalyticsListDeleted, map[string]interface{}{
		"list_id": listId,
	})
	return nil
}

func (usecase *CustomListUseCase) GetCustomListById(ctx context.Context, id string) (models.CustomList, error) {
	customList, err := usecase.CustomListRepository.GetCustomListById(ctx,
		usecase.executorFactory.NewExecutor(), id)
	if err != nil {
		return models.CustomList{}, err
	}
	if err := usecase.enforceSecurity.ReadCustomList(customList); err != nil {
		return models.CustomList{}, err
	}
	return customList, nil
}

func (usecase *CustomListUseCase) GetCustomListValues(ctx context.Context,
	getCustomListValues models.GetCustomListValuesInput,
) ([]models.CustomListValue, error) {
	// CustomListValues doesn't know the OrganizationId of the list
	// so we call GetCustomListById so that it check if we are allowed to read it
	if _, err := usecase.GetCustomListById(ctx, getCustomListValues.Id); err != nil {
		return []models.CustomListValue{}, err
	}
	return usecase.CustomListRepository.GetCustomListValues(ctx,
		usecase.executorFactory.NewExecutor(), getCustomListValues)
}

func (usecase *CustomListUseCase) AddCustomListValue(ctx context.Context,
	addCustomListValue models.AddCustomListValueInput,
) (models.CustomListValue, error) {
	var userId *models.UserId
	creds, found := utils.CredentialsFromCtx(ctx)
	if found {
		userId = &creds.ActorIdentity.UserId
	}

	value, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.CustomListValue, error) {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, addCustomListValue.CustomListId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return models.CustomListValue{}, err
		}
		newCustomListValueId := uuid.NewString()

		err = usecase.CustomListRepository.AddCustomListValue(ctx, tx, addCustomListValue, newCustomListValueId, userId)
		if err != nil {
			return models.CustomListValue{}, err
		}
		return usecase.CustomListRepository.GetCustomListValueById(ctx, tx, newCustomListValueId)
	})
	if err != nil {
		return models.CustomListValue{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsListValueCreated, map[string]interface{}{
		"list_id": addCustomListValue.CustomListId,
	})

	return value, nil
}

func (usecase *CustomListUseCase) ReadCustomListValuesToCSV(ctx context.Context, customListID string, w io.Writer) (string, error) {
	exec := usecase.executorFactory.NewExecutor()
	customList, err := usecase.CustomListRepository.GetCustomListById(ctx, exec, customListID)
	if err != nil {
		return "", err
	}
	if err := usecase.enforceSecurity.ReadCustomList(customList); err != nil {
		return "", err
	}

	customListValues, err := usecase.CustomListRepository.GetCustomListValues(ctx, exec, models.GetCustomListValuesInput{
		Id: customListID,
	})
	if err != nil {
		return "", err
	}

	csvWriter := csv.NewWriter(w)
	for _, customListValue := range customListValues {
		if err := csvWriter.Write([]string{customListValue.Value}); err != nil {
			return "", err
		}
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		return "", err
	}

	return customList.Name, nil
}

func (usecase *CustomListUseCase) ReplaceCustomListValuesFromCSV(ctx context.Context, customListID string, fileReader *csv.Reader) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Ingesting custom list values from CSV")
	total := 0
	start := time.Now()
	printDuration := func() {
		end := time.Now()
		duration := end.Sub(start)
		// divide by 1e6 convert to milliseconds (base is nanoseconds)
		avgDuration := float64(duration) / float64(total*1e6)
		logger.InfoContext(ctx, fmt.Sprintf("Successfully ingested %d custom list values in %s, average %vms", total, duration, avgDuration))
	}
	defer printDuration()

	var userId *models.UserId
	creds, found := utils.CredentialsFromCtx(ctx)
	if found {
		userId = &creds.ActorIdentity.UserId
	}

	customListValuesFromCSV, err := processCSVFile(fileReader)
	if err != nil {
		return errors.Wrap(models.BadParameterError, err.Error())
	}

	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, customListID)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}

		currentCustomListValues, err := usecase.CustomListRepository.GetCustomListValues(
			ctx, tx, models.GetCustomListValuesInput{Id: customListID}, true)
		if err != nil {
			return err
		}

		newCustomListValuesToAdd, currentCustomListValueIdsToDelete :=
			computeCustomListValueUpdates(customListValuesFromCSV, currentCustomListValues)

		newCustomListValuesInput := make([]models.BatchInsertCustomListValue, len(newCustomListValuesToAdd))
		for i, customListValue := range newCustomListValuesToAdd {
			newCustomListValuesInput[i] = models.BatchInsertCustomListValue{
				Id:    uuid.NewString(),
				Value: customListValue,
			}
		}

		err = usecase.CustomListRepository.BatchInsertCustomListValues(ctx, tx,
			customListID, newCustomListValuesInput, userId)
		if err != nil {
			return err
		}

		err = usecase.CustomListRepository.BatchDeleteCustomListValues(ctx, tx,
			customListID, currentCustomListValueIdsToDelete, userId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsListValueCreated, map[string]interface{}{
		"list_id": customListID,
	})

	return nil
}

func computeCustomListValueUpdates(customListValuesFromCSV []string,
	currentCustomListValues []models.CustomListValue,
) ([]string, []string) {
	newCustomListValuesMap := make(map[string]bool)
	for _, customListValue := range customListValuesFromCSV {
		newCustomListValuesMap[customListValue] = true
	}

	currentCustomListValueIdsToDelete := make([]string, 0)
	for _, customListValue := range currentCustomListValues {
		_, ok := newCustomListValuesMap[customListValue.Value]
		if ok {
			newCustomListValuesMap[customListValue.Value] = false
		} else {
			currentCustomListValueIdsToDelete = append(
				currentCustomListValueIdsToDelete, customListValue.Id)
		}
	}

	newCustomListValuesToAdd := make([]string, 0)
	for customListValue, shouldAdd := range newCustomListValuesMap {
		if shouldAdd {
			newCustomListValuesToAdd = append(newCustomListValuesToAdd, customListValue)
		}
	}
	return newCustomListValuesToAdd, currentCustomListValueIdsToDelete
}

var maxCustomListValues = 10000

func processCSVFile(fileReader *csv.Reader) ([]string, error) {
	customListValues := make([]string, 0)
	for lineNumber := 1; ; lineNumber++ {
		row, err := fileReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			var parseError *csv.ParseError
			if errors.As(err, &parseError) {
				return nil, err
			} else {
				return nil, fmt.Errorf("error found at line %d in CSV", lineNumber)
			}
		}

		if len(row) != 1 {
			return nil, fmt.Errorf("invalid CSV row: expected 1 column, got %v at line %d",
				len(row), lineNumber)
		}
		customListValues = append(customListValues, row[0])
	}
	if len(customListValues) > maxCustomListValues {
		return nil, fmt.Errorf("too many values in CSV: expected at most %v, got %v",
			maxCustomListValues, len(customListValues))
	}
	return customListValues, nil
}

func (usecase *CustomListUseCase) DeleteCustomListValue(ctx context.Context,
	deleteCustomListValue models.DeleteCustomListValueInput,
) error {
	var userId *models.UserId
	creds, found := utils.CredentialsFromCtx(ctx)
	if found {
		userId = &creds.ActorIdentity.UserId
	}

	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		customList, err := usecase.CustomListRepository.GetCustomListById(ctx, tx, deleteCustomListValue.CustomListId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.ModifyCustomList(customList); err != nil {
			return err
		}
		return usecase.CustomListRepository.DeleteCustomListValue(ctx, tx, deleteCustomListValue, userId)
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsListValueDeleted, map[string]interface{}{
		"list_id": deleteCustomListValue.CustomListId,
	})

	return nil
}
