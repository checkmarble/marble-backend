package api

import (
	"fmt"
	"marble/marble-backend/server/dto"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetScheduledExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id, err := requiredUuidUrlParam(r, "scheduledExecutionID")
		if utils.PresentError(w, r, err) {
			return
		}
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if utils.PresentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledExecutionUsecase()
		execution, err := usecase.GetScheduledExecution(ctx, organizationId, id)

		if utils.PresentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scheduled_execution", dto.AdaptScheduledExecutionDto(execution))
	}
}

func (api *API) handleListScheduledExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.ListScheduledExecutionInput)
		scenarioId := input.ScenarioID

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if utils.PresentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScheduledExecutionUsecase()
		executions, err := usecase.ListScheduledExecutions(ctx, organizationId, scenarioId)

		if utils.PresentError(w, r, err) {
			fmt.Println(err)
			return
		}

		PresentModelWithName(w, "scheduled_executions", utils.Map(executions, dto.AdaptScheduledExecutionDto))
	}
}
