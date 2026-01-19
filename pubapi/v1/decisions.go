package v1

import (
	"time"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p params.ListDecisionsParams

		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !p.StartDate.IsZero() && !p.EndDate.IsZero() {
			if time.Time(p.StartDate).After(time.Time(p.EndDate)) {
				types.NewErrorResponse().WithError(errors.WithDetail(
					types.ErrInvalidPayload, "end date should be after start date")).Serve(c)
				return
			}
			if time.Time(p.StartDate).Before(time.Time(p.EndDate).Add(-params.MAX_DECISIONS_DATE_RANGE)) {
				types.NewErrorResponse().WithError(errors.WithDetailf(
					types.ErrInvalidPayload, "start and end date must be at most %.0f days apart",
					params.MAX_DECISIONS_DATE_RANGE.Hours()/24)).Serve(c)
				return
			}
		}

		filters := p.ToFilters()
		paging := p.PaginationParams.ToModel(decisionPaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decisions, err := decisionsUsecase.ListDecisions(ctx, orgId, paging, filters)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(decisions.Decisions) > 0 {
			nextPageId = decisions.Decisions[len(decisions.Decisions)-1].DecisionId.String()
		}

		types.
			NewResponse(pure_utils.Map(decisions.Decisions, dto.AdaptDecision(false, nil, nil))).
			WithPagination(decisions.HasNextPage, nextPageId).
			Serve(c)
	}
}

func HandleGetDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		decisionId, err := types.UuidParam(c, "decisionId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		decisionsUsecase := uc.NewDecisionUsecase()

		decision, err := decisionsUsecase.GetDecision(ctx, decisionId.String())
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.
			NewResponse(dto.AdaptDecision(true, decision.RuleExecutions,
				decision.ScreeningExecutions)(decision.Decision)).
			Serve(c)
	}
}

func HandleCreateDecision(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateDecisionParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		scenariosUsecase := uc.NewScenarioUsecase()
		decisionsUsecase := uc.NewDecisionUsecase()

		scenario, err := scenariosUsecase.GetScenario(ctx, payload.ScenarioId)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				types.NewErrorResponse().WithError(err).WithErrorMessage("scenario was not found").Serve(c)
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
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
				WithDisallowUnknownFields:   true,
			},
		)
		if err != nil {
			if presentDecisionCreationError(c, err) {
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		stats := gdto.DecisionsAggregateMetadata{}

		if !triggerPassed {
			stats.Count.Skipped = 1

			types.
				NewResponse([]struct{}{}).
				WithMetadata(dto.AdaptDecisionsMetadata(stats)).
				Serve(c)
			return
		}

		stats.Count.Total = 1

		switch decision.Outcome {
		case models.Approve:
			stats.Count.Approve = 1
		case models.Review:
			stats.Count.Review = 1
		case models.BlockAndReview:
			stats.Count.BlockAndReview = 1
		case models.Decline:
			stats.Count.Decline = 1
		}

		types.
			NewResponse([]dto.Decision{dto.AdaptDecision(true, decision.RuleExecutions,
				decision.ScreeningExecutions)(decision.Decision)}).
			WithMetadata(dto.AdaptDecisionsMetadata(stats)).
			Serve(c)
	}
}

func HandleCreateAllDecisions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var payload params.CreateAllDecisionsParams

		if err := c.ShouldBindJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
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
			models.CreateDecisionParams{
				WithScenarioPermissionCheck: true,
				WithDecisionWebhooks:        true,
				WithRuleExecutionDetails:    true,
				WithDisallowUnknownFields:   true,
			},
		)
		if err != nil {
			if presentDecisionCreationError(c, err) {
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		dtos := pure_utils.Map(decisions, func(d models.DecisionWithRuleExecutions) dto.Decision {
			return dto.AdaptDecision(true, d.RuleExecutions, d.ScreeningExecutions)(d.Decision)
		})

		stats := gdto.AdaptDecisionsMetadata(decisions, skipped)

		types.NewResponse(dtos).WithMetadata(dto.AdaptDecisionsMetadata(stats)).Serve(c)
	}
}

type AddDecisionToCaseParams struct {
	CaseId uuid.UUID `json:"case_id" binding:"required"`
}

func HandleAddDecisionToCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		decisionId, err := types.UuidParam(c, "decisionId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params AddDecisionToCaseParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		caseUsecase := uc.NewCaseUseCase()
		decisionUsecase := uc.NewDecisionUsecase()

		_, err = caseUsecase.AddDecisionsToCase(ctx, "", params.CaseId.String(), []string{decisionId.String()})
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		decision, err := decisionUsecase.GetDecision(ctx, decisionId.String())
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptDecision(false, nil, nil)(decision.Decision)).Serve(c)
	}
}

func presentDecisionCreationError(c *gin.Context, err error) bool {
	var validationError models.IngestionValidationErrors

	if errors.As(err, &validationError) {
		_, errs := validationError.GetSomeItem()

		types.NewErrorResponse().
			WithError(errs).
			WithErrorCode(string(gdto.SchemaMismatchError)).
			WithErrorMessage("the provided trigger object is invalid").
			WithErrorDetails(errs).
			Serve(c)

		return true
	}

	return false
}
