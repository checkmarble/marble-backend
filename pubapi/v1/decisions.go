package v1

import (
	"fmt"
	"time"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

var decisionPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.DecisionSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func HandleListDecisions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params params.ListDecisionsParams

		if err := c.ShouldBindQuery(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !params.StartDate.IsZero() && !params.EndDate.IsZero() {
			if time.Time(params.StartDate).After(time.Time(params.EndDate)) {
				pubapi.NewErrorResponse().WithError(errors.WithDetail(pubapi.ErrInvalidPayload, "end date should be after start date")).Serve(c)
				return
			}
		}

		filters := params.ToFilters()
		paging := params.PaginationParams.ToModel(decisionPaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decisions, err := decisionsUsecase.ListDecisions(ctx, orgId, paging, filters)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(decisions.Decisions) > 0 {
			nextPageId = decisions.Decisions[len(decisions.Decisions)-1].DecisionId
		}

		pubapi.
			NewResponse(pure_utils.Map(decisions.Decisions, dto.AdaptDecision(nil))).
			WithPagination(decisions.HasNextPage, nextPageId).
			Serve(c)
	}
}

func HandleGetDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		decisionId := c.Param("decisionId")

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decision, err := decisionsUsecase.GetDecision(ctx, decisionId)
		fmt.Printf("%#v\n", decision)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptDecision(decision.RuleExecutions)(decision.Decision)).
			Serve(c)
	}
}

func HandleCreateDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateDecisionParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		scenariosUsecase := uc.NewScenarioUsecase()
		decisionsUsecase := uc.NewDecisionUsecase()

		scenario, err := scenariosUsecase.GetScenario(ctx, payload.ScenarioId)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				pubapi.NewErrorResponse().WithError(err).WithErrorMessage("scenario was not found").Serve(c)
				return
			}

			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		triggerPassed, decision, err := decisionsUsecase.CreateDecision(
			ctx,
			models.CreateDecisionInput{
				OrganizationId:     orgId,
				ScenarioId:         payload.ScenarioId,
				TriggerObjectTable: scenario.TriggerObjectType,
				PayloadRaw:         payload.TriggerObject,
			},
			models.CreateDecisionParams{
				WithScenarioPermissionCheck: true,
				WithDecisionWebhooks:        true,
				WithRuleExecutionDetails:    true,
			},
		)

		if err != nil {
			if presentDecisionCreationError(c, err) {
				return
			}

			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !triggerPassed {
			pubapi.NewErrorResponse().
				WithErrorCode(string(gdto.TriggerConditionNotMatched)).
				WithError(errors.WithDetail(pubapi.ErrUnprocessableEntity, "the payload object you sent does not match the trigger condition of the scenario")).
				Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptDecision(decision.RuleExecutions)(decision.Decision)).Serve(c)
	}
}

func HandleCreateAllDecisions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateAllDecisionsParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decisions, skipped, err := decisionsUsecase.CreateAllDecisions(
			ctx,
			models.CreateAllDecisionsInput{
				OrganizationId:     orgId,
				TriggerObjectTable: payload.TriggerObjectType,
				PayloadRaw:         payload.TriggerObject,
			},
		)

		if err != nil {
			if presentDecisionCreationError(c, err) {
				return
			}

			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		dtos := pure_utils.Map(decisions, func(d models.DecisionWithRuleExecutions) dto.Decision {
			return dto.AdaptDecision(d.RuleExecutions)(d.Decision)
		})

		stats := gdto.AdaptDecisionsMetadata(decisions, skipped)

		pubapi.NewResponse(dtos).WithMetadata(dto.AdaptDecisionsMetadata(stats)).Serve(c)
	}
}

func presentDecisionCreationError(c *gin.Context, err error) bool {
	var validationError models.IngestionValidationErrors

	if errors.As(err, &validationError) {
		_, errs := validationError.GetSomeItem()

		details := make([]string, 0, len(errs))

		for field, fieldErr := range errs {
			details = append(details, fmt.Sprintf("%s %s", field, fieldErr))
		}

		pubapi.NewErrorResponse().
			WithError(errs).
			WithErrorCode(string(gdto.SchemaMismatchError)).
			WithErrorDetails(details...).
			Serve(c)

		return true
	}

	return false
}
