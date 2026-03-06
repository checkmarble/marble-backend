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

type scenarioReader interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
}

type asyncDecisionsSecurity interface {
	ReadScenario(scenario models.Scenario) error
}

type AsyncDecisionExecutionUsecase struct {
	executorFactory                  executor_factory.ExecutorFactory
	transactionFactory               executor_factory.TransactionFactory
	enforceSecurityDecisions         security.EnforceSecurityDecision
	enforceSecurity                  asyncDecisionsSecurity
	asyncDecisionExecutionRepository repositories.AsyncDecisionExecutionRepository
	taskQueueRepository              repositories.TaskQueueRepository
	dataModelRepository              repositories.DataModelRepository
	scenarioReader                   scenarioReader
}

func NewAsyncDecisionExecutionUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurityDecisions security.EnforceSecurityDecision,
	enforceSecurity asyncDecisionsSecurity,
	asyncDecisionExecutionRepository repositories.AsyncDecisionExecutionRepository,
	taskQueueRepository repositories.TaskQueueRepository,
	dataModelRepository repositories.DataModelRepository,
	scenarioReader scenarioReader,
) *AsyncDecisionExecutionUsecase {
	return &AsyncDecisionExecutionUsecase{
		executorFactory:                  executorFactory,
		transactionFactory:               transactionFactory,
		enforceSecurityDecisions:         enforceSecurityDecisions,
		enforceSecurity:                  enforceSecurity,
		asyncDecisionExecutionRepository: asyncDecisionExecutionRepository,
		taskQueueRepository:              taskQueueRepository,
		dataModelRepository:              dataModelRepository,
		scenarioReader:                   scenarioReader,
	}
}

func (usecase *AsyncDecisionExecutionUsecase) CreateAsyncDecisionExecution(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	objects []json.RawMessage,
	scenarioId *string,
	shouldIngest bool,
) ([]models.AsyncDecisionExecution, error) {
	exec := usecase.executorFactory.NewExecutor()
	if err := usecase.enforceSecurityDecisions.CreateDecision(orgId); err != nil {
		return nil, err
	}
	if scenarioId != nil {
		scenario, err := usecase.scenarioReader.GetScenarioById(ctx, exec, *scenarioId)
		if err != nil {
			return nil, errors.Wrap(err, "error looking up scenario for batch async decision execution")
		}
		if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
			return nil, err
		}
	}

	objectType, err := usecase.resolveObjectType(ctx, orgId, objectType, scenarioId)
	if err != nil {
		return nil, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, orgId, false, true)
	if err != nil {
		return nil, errors.Wrap(err,
			"error getting data model in validatePayload")
	}
	// Validate all payloads upfront, collecting all errors
	validationErrors := make(models.IngestionValidationErrors)
	for _, obj := range objects {
		err := usecase.validatePayload(ctx, objectType, obj, dataModel)
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
					ScenarioId:    scenarioId,
					ShouldIngest:  shouldIngest,
				}
			}

			if err := usecase.asyncDecisionExecutionRepository.CreateAsyncDecisionExecutions(
				ctx, tx, createInputs,
			); err != nil {
				return nil, errors.Wrap(err,
					"error creating async decision execution batch")
			}

			if err := usecase.taskQueueRepository.EnqueueAsyncDecisionExecutions(
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
					ScenarioId:    scenarioId,
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
	if err := usecase.enforceSecurityDecisions.ReadDecision(models.Decision{
		OrganizationId: execution.OrgId,
	}); err != nil {
		return models.AsyncDecisionExecution{}, err
	}

	return execution, nil
}

// resolveObjectType ensures we have a trigger_object_type. When only scenario_id is provided,
// it looks up the scenario to derive the object type.
func (usecase *AsyncDecisionExecutionUsecase) resolveObjectType(
	ctx context.Context,
	orgId uuid.UUID,
	objectType string,
	scenarioId *string,
) (string, error) {
	if objectType == "" && scenarioId == nil {
		return "", errors.Wrap(models.BadParameterError,
			"one of trigger_object_type or scenario_id is required")
	}

	if objectType != "" {
		return objectType, nil
	}

	// Derive object type from scenario
	exec := usecase.executorFactory.NewExecutor()
	scenario, err := usecase.scenarioReader.GetScenarioById(ctx, exec, *scenarioId)
	if err != nil {
		return "", errors.Wrap(err, "error looking up scenario for trigger_object_type")
	}
	if scenario.OrganizationId != orgId {
		return "", errors.Wrap(models.NotFoundError, "scenario not found")
	}

	return scenario.TriggerObjectType, nil
}

// validatePayload validates the trigger object payload against the data model schema.
func (usecase *AsyncDecisionExecutionUsecase) validatePayload(
	ctx context.Context,
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
