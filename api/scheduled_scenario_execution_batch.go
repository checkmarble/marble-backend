package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

// //////////////////////////////////
// At the batch level
// //////////////////////////////////
func (api *API) handleGetScheduledScenarioExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := requiredUuidUrlParam(r, "scheduledScenarioExecutionID")
		if presentError(w, r, err) {
			return
		}
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledScenarioExecutionUsecase()
		executionBatch, err := usecase.GetScheduledScenarioExecution(ctx, organizationId, id)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scenario_batch_execution", dto.AdaptBatchExecutionDto(executionBatch))
	}
}

func (api *API) handleListScheduledScenarioExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.ListScheduledScenarioExecutionInput)
		scenarioId := input.ScenarioID

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledScenarioExecutionUsecase()
		executions, err := usecase.ListScheduledScenarioExecutions(ctx, organizationId, scenarioId)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scenario_batch_executions", utils.Map(executions, dto.AdaptBatchExecutionDto))
	}
}

func (api *API) handleUpdateScheduledScenarioExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		input := ctx.Value(httpin.Input).(*dto.UpdateScheduledScenarioExecutionInput)
		id := input.ID
		body := input.Body

		usecase := api.usecases.NewScheduledScenarioExecutionUsecase()
		executionBatch, err := usecase.UpdateScheduledScenarioExecutioExecution(ctx, organizationId, id, dto.AdaptBatchExecutionUpdateBody(*body))

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scenario_batch_execution", dto.AdaptBatchExecutionDto(executionBatch))
	}
}

// //////////////////////////////////
// At the object level
// //////////////////////////////////

func (api *API) handleGetScheduledScenarioObjectExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := requiredUuidUrlParam(r, "scheduledScenarioObjectExecutionID")
		if presentError(w, r, err) {
			return
		}
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledScenarioExecutionUsecase()
		execution, err := usecase.GetScheduledScenarioObjectExecution(ctx, organizationId, id)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scenario_batch_execution_object", dto.AdaptBatchExecutionObjectDto(execution))
	}
}

func (api *API) handleListScheduledScenarioObjectExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.ListScheduledScenarioObjectExecutionInput)
		batchId := input.ScenarioBatchExecutionID

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledScenarioExecutionUsecase()
		executions, err := usecase.ListScheduledScenarioObjectExecutions(ctx, organizationId, batchId)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scenario_batch_execution_objects", utils.Map(executions, dto.AdaptBatchExecutionObjectDto))
	}
}
