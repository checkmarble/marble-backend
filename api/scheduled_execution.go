package api

import (
	"net/http"
	"strconv"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetScheduledExecution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scheduledExecutionID, err := requiredUuidUrlParam(r, "scheduledExecutionID")
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewScheduledExecutionUsecase()
		execution, err := usecase.GetScheduledExecution(scheduledExecutionID)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scheduled_execution", dto.AdaptScheduledExecutionDto(execution))
	}
}

func (api *API) handleGetScheduledExecutionDecisions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scheduledExecutionID, err := requiredUuidUrlParam(r, "scheduledExecutionID")
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewScheduledExecutionUsecase()

		w.Header().Set("Content-Type", "application/x-ndjson")
		// TODO: better filename
		w.Header().Set("Content-Disposition", "attachment; filename=decisions.ndjson")
		number_of_exported_decisions, err := usecase.ExportScheduledExecutionDecisions(scheduledExecutionID, w)

		if err != nil {
			// note: un case of security error, the header has not been sent, so we can still send a 401
			presentError(w, r, err)
			return
		}

		// nice trailer
		w.Header().Set("X-NUMBER-OF-DECISIONS", strconv.Itoa(number_of_exported_decisions))
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

		usecase := api.UsecasesWithCreds(r).NewScheduledExecutionUsecase()
		executions, err := usecase.ListScheduledExecutions(organizationId, scenarioId)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "scheduled_executions", utils.Map(executions, dto.AdaptScheduledExecutionDto))
	}
}
