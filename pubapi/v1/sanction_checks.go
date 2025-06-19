package v1

import (
	"encoding/json"
	"net/http"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func HandleListScreenings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		decisionId, err := pubapi.UuidParam(c, "decisionId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		screeningUsecase := uc.NewScreeningUsecase()

		if !pubapi.CheckFeatureAccess(c, uc) {
			return
		}

		sc, err := screeningUsecase.ListScreenings(c.Request.Context(), decisionId.String(), false)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if len(sc) == 0 {
			pubapi.NewErrorResponse().WithError(errors.WithDetail(models.NotFoundError,
				"this decision does not have a sanction check")).Serve(c)
			return
		}

		pubapi.
			NewResponse(pure_utils.Map(sc, dto.AdaptScreening(true))).
			WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId.String()}).
			Serve(c)
	}
}

type UpdateScreeningMatchStatusParams struct {
	Status    string `json:"status" binding:"required,oneof=no_hit confirmed_hit"`
	Whitelist bool   `json:"whitelist" binding:"excluded_unless=Status no_hit"`
}

func HandleUpdateScreeningMatchStatus(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		matchId, err := pubapi.UuidParam(c, "matchId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params UpdateScreeningMatchStatusParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		screeningUsecase := uc.NewScreeningUsecase()

		match, err := screeningUsecase.UpdateMatchStatus(c.Request.Context(), models.ScreeningMatchUpdate{
			MatchId:   matchId.String(),
			Status:    models.ScreeningMatchStatusFrom(params.Status),
			Whitelist: params.Whitelist,
		})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptScreeningMatch(match)).
			WithLink(pubapi.LinkScreenings, gin.H{"id": match.ScreeningId}).
			Serve(c)
	}
}

func HandleRefineScreening(uc usecases.Usecases, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		screeningId, err := pubapi.UuidParam(c, "screeningId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params gdto.RefineQueryDto

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if !params.Validate() {
			pubapi.
				NewErrorResponse().
				WithErrorMessage("refine query is missing some required fields").
				Serve(c, http.StatusBadRequest)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		screeningUsecase := uc.NewScreeningUsecase()

		screening, err := screeningUsecase.GetScreening(c.Request.Context(), screeningId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		refineQuery := models.ScreeningRefineRequest{
			DecisionId: screening.DecisionId,
			Type:       params.Type(),
			Query:      gdto.AdaptRefineQueryDto(params),
		}

		switch write {
		case true:
			screening, err := screeningUsecase.Refine(c.Request.Context(), refineQuery, nil)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

			pubapi.
				NewResponse(dto.AdaptScreening(true)(screening)).
				WithLink(pubapi.LinkDecisions, gin.H{pubapi.LinkDecisions: screening.DecisionId}).
				Serve(c)

		case false:
			screening, err := screeningUsecase.Search(c.Request.Context(), refineQuery)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

			matchPayload := func(m models.ScreeningMatch) json.RawMessage {
				return m.Payload
			}

			pubapi.
				NewResponse(pure_utils.Map(screening.Matches, matchPayload)).
				WithLink(pubapi.LinkDecisions, gin.H{pubapi.LinkDecisions: screening.DecisionId}).
				Serve(c)
		}
	}
}

func HandleScreeningFreeformSearch(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params gdto.RefineQueryDto

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if !params.Validate() {
			pubapi.
				NewErrorResponse().
				WithErrorMessage("refine query is missing some required fields").
				Serve(c, http.StatusBadRequest)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		screeningUsecase := uc.NewScreeningUsecase()

		refineQuery := models.ScreeningRefineRequest{
			Type:  params.Type(),
			Query: gdto.AdaptRefineQueryDto(params),
		}

		screening, err := screeningUsecase.FreeformSearch(c.Request.Context(),
			orgId, models.ScreeningConfig{}, refineQuery)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		matchPayload := func(m models.ScreeningMatch) json.RawMessage {
			return m.Payload
		}

		pubapi.
			NewResponse(pure_utils.Map(screening.Matches, matchPayload)).
			Serve(c)
	}
}

func HandleGetScreeningEntity(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		entityId := c.Param("entityId")

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		screeningUsecase := uc.NewScreeningUsecase()

		entity, err := screeningUsecase.GetEntity(ctx, entityId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(json.RawMessage(entity)).Serve(c)
	}
}

type AddWhitelistParams struct {
	Counterparty string `json:"counterparty" binding:"required"`
	EntityId     string `json:"entity_id" binding:"required"`
}

func HandleAddWhitelist(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params AddWhitelistParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		screeningUsecase := uc.NewScreeningUsecase()

		if err := screeningUsecase.CreateWhitelist(ctx, nil, orgId,
			params.Counterparty, params.EntityId, nil); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusCreated)
	}
}

type DeleteWhitelistParams struct {
	Counterparty *string `json:"counterparty"`
	EntityId     string  `json:"entity_id" binding:"required"`
}

func HandleDeleteWhitelist(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params DeleteWhitelistParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		screeningUsecase := uc.NewScreeningUsecase()

		if err := screeningUsecase.DeleteWhitelist(ctx, nil, orgId,
			params.Counterparty, params.EntityId, nil); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

type SearchWhitelistParams struct {
	Counterparty *string `json:"counterparty"`
	EntityId     *string `json:"entity_id"`
}

func HandleSearchWhitelist(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params SearchWhitelistParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if params.Counterparty == nil && params.EntityId == nil {
			pubapi.
				NewErrorResponse().
				WithError(errors.WithDetail(models.BadParameterError,
					"at least one of `counterparty` or `entity_id` must be provided")).
				Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		screeningUsecase := uc.NewScreeningUsecase()

		whitelists, err := screeningUsecase.SearchWhitelist(ctx, nil,
			orgId, params.Counterparty, params.EntityId, nil)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(pure_utils.Map(whitelists, dto.AdaptScreeningWhitelist)).
			Serve(c)
	}
}
