package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetScheduledExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := requiredUuidUrlParam(r, "scheduledExecutionID")
		if presentError(w, r, err) {
			return
		}
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledExecutionUsecase()
		execution, err := usecase.GetScheduledExecution(ctx, organizationId, id)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scheduled_execution", dto.AdaptScheduledExecutionDto(execution))
	}
}

func (api *API) handleListScheduledExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.ListScheduledExecutionInput)
		scenarioId := input.ScenarioId

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledExecutionUsecase()
		executions, err := usecase.ListScheduledExecutions(ctx, organizationId, scenarioId)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scheduled_executions", utils.Map(executions, dto.AdaptScheduledExecutionDto))
	}
}
