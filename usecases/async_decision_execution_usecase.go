package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type AsyncDecisionExecutionUsecase struct {
	executorFactory                  executor_factory.ExecutorFactory
	transactionFactory               executor_factory.TransactionFactory
	enforceSecurity                  security.EnforceSecurityDecision
	asyncDecisionExecutionRepository repositories.AsyncDecisionExecutionRepository
	taskQueueRepository              repositories.TaskQueueRepository
	dataModelRepository              repositories.DataModelRepository
}

func (usecase *AsyncDecisionExecutionUsecase) CreateAsyncDecisionExecution(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	triggerObject json.RawMessage,
	shouldIngest bool,
) (models.AsyncDecisionExecution, error) {
	if err := usecase.enforceSecurity.CreateDecision(orgId); err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx,
		usecase.executorFactory.NewExecutor(), orgId, false, true)
	if err != nil {
		return models.AsyncDecisionExecution{}, errors.Wrap(err,
			"error getting data model in validatePayload")
	}
	if err := usecase.validatePayload(ctx, orgId, objectType, triggerObject, dataModel); err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	executionId := uuid.Must(uuid.NewV7())

	execution, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.AsyncDecisionExecution, error) {
			createInput := models.AsyncDecisionExecutionCreate{
				Id:            executionId,
				OrgId:         orgId,
				ObjectType:    objectType,
				TriggerObject: triggerObject,
				ShouldIngest:  shouldIngest,
			}

			if err := usecase.asyncDecisionExecutionRepository.CreateAsyncDecisionExecution(
				ctx, tx, createInput,
			); err != nil {
				return models.AsyncDecisionExecution{}, errors.Wrap(err,
					"error creating async decision execution")
			}

			if err := usecase.taskQueueRepository.EnqueueAsyncDecisionExecution(
				ctx, tx, orgId, executionId,
			); err != nil {
				return models.AsyncDecisionExecution{}, errors.Wrap(err,
					"error enqueuing async decision execution")
			}

			return models.AsyncDecisionExecution{
				Id:            executionId,
				OrgId:         orgId,
				ObjectType:    objectType,
				TriggerObject: triggerObject,
				ShouldIngest:  shouldIngest,
				Status:        models.AsyncDecisionExecutionStatusPending,
			}, nil
		},
	)
	if err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	return execution, nil
}

func (usecase *AsyncDecisionExecutionUsecase) CreateAsyncDecisionExecutionBatch(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objects []json.RawMessage,
	shouldIngest bool,
) ([]models.AsyncDecisionExecution, error) {
	if err := usecase.enforceSecurity.CreateDecision(orgId); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, orgId, false, true)
	if err != nil {
		return nil, errors.Wrap(err,
			"error getting data model in validatePayload")
	}
	// Validate all payloads upfront, collecting all errors
	validationErrors := make(models.IngestionValidationErrors)
	for _, obj := range objects {
		err := usecase.validatePayload(ctx, orgId, objectType, obj, dataModel)
		var objErrors models.IngestionValidationErrors
		if errors.As(err, &objErrors) {
			objectId, errMap := objErrors.GetSomeItem()
			validationErrors[objectId] = errMap
		} else if err != nil {
			return nil, err
		}
	}
	if len(validationErrors) > 0 {
		return nil, validationErrors
	}

	// Generate IDs for all executions
	executionIds := make([]uuid.UUID, len(objects))
	for i := range objects {
		executionIds[i] = uuid.Must(uuid.NewV7())
	}

	executions, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) ([]models.AsyncDecisionExecution, error) {
			createInputs := make([]models.AsyncDecisionExecutionCreate, len(objects))
			for i, obj := range objects {
				createInputs[i] = models.AsyncDecisionExecutionCreate{
					Id:            executionIds[i],
					OrgId:         orgId,
					ObjectType:    objectType,
					TriggerObject: obj,
					ShouldIngest:  shouldIngest,
				}
			}

			if err := usecase.asyncDecisionExecutionRepository.CreateAsyncDecisionExecutionBatch(
				ctx, tx, createInputs,
			); err != nil {
				return nil, errors.Wrap(err,
					"error creating async decision execution batch")
			}

			if err := usecase.taskQueueRepository.EnqueueAsyncDecisionExecutionBatch(
				ctx, tx, orgId, executionIds,
			); err != nil {
				return nil, errors.Wrap(err,
					"error enqueuing async decision execution batch")
			}

			result := make([]models.AsyncDecisionExecution, len(objects))
			for i, obj := range objects {
				result[i] = models.AsyncDecisionExecution{
					Id:            executionIds[i],
					OrgId:         orgId,
					ObjectType:    objectType,
					TriggerObject: obj,
					ShouldIngest:  shouldIngest,
					Status:        models.AsyncDecisionExecutionStatusPending,
				}
			}
			return result, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return executions, nil
}

func (usecase *AsyncDecisionExecutionUsecase) GetAsyncDecisionExecution(
	ctx context.Context,
	executionId uuid.UUID,
) (models.AsyncDecisionExecution, error) {
	exec := usecase.executorFactory.NewExecutor()
	execution, err := usecase.asyncDecisionExecutionRepository.GetAsyncDecisionExecution(
		ctx, exec, executionId,
	)
	if err != nil {
		return models.AsyncDecisionExecution{}, errors.Wrap(err,
			"error getting async decision execution")
	}

	// Enforce permission + org ownership (same pattern as GetDecision)
	if err := usecase.enforceSecurity.ReadDecision(models.Decision{
		OrganizationId: execution.OrgId,
	}); err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	return execution, nil
}

// validatePayload validates the trigger object payload against the data model schema.
func (usecase *AsyncDecisionExecutionUsecase) validatePayload(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	rawPayload json.RawMessage,
	dm models.DataModel,
) error {
	if len(rawPayload) == 0 {
		return errors.Wrap(models.BadParameterError, "empty payload received")
	}

	table, ok := dm.Tables[objectType]
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("table %s not found in data model", objectType),
		)
	}

	parser := payload_parser.NewParser(payload_parser.DisallowUnknownFields())
	if _, err := parser.ParsePayload(ctx, table, rawPayload); err != nil {
		return err
	}

	return nil
}
