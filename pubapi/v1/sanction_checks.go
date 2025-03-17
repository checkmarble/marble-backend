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

func HandleListSanctionChecks(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		decisionId, err := pubapi.UuidParam(c, "decisionId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		if !pubapi.CheckFeatureAccess(c, uc) {
			return
		}

		sc, err := sanctionCheckUsecase.ListSanctionChecks(c.Request.Context(), decisionId.String(), false)
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
			NewResponse(pure_utils.Map(sc, dto.AdaptSanctionCheck)).
			WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId.String()}).
			Serve(c)
	}
}

type UpdateSanctionCheckMatchStatusParams struct {
	Status    string `json:"status" binding:"required,oneof=no_hit confirmed_hit"`
	Whitelist bool   `json:"whitelist" binding:"excluded_unless=Status no_hit"`
}

func HandleUpdateSanctionCheckMatchStatus(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		matchId, err := pubapi.UuidParam(c, "matchId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params UpdateSanctionCheckMatchStatusParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		match, err := sanctionCheckUsecase.UpdateMatchStatus(c.Request.Context(), models.SanctionCheckMatchUpdate{
			MatchId:   matchId.String(),
			Status:    models.SanctionCheckMatchStatusFrom(params.Status),
			Whitelist: params.Whitelist,
		})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptSanctionCheckMatch(match)).
			WithLink(pubapi.LinkSanctionChecks, gin.H{"id": match.SanctionCheckId}).
			Serve(c)
	}
}

func HandleRefineSanctionCheck(uc usecases.Usecases, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		decisionId, err := pubapi.UuidParam(c, "decisionId")
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
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		refineQuery := models.SanctionCheckRefineRequest{
			DecisionId: decisionId.String(),
			Type:       params.Type(),
			Query:      gdto.AdaptRefineQueryDto(params),
		}

		switch write {
		case true:
			sanctionCheck, err := sanctionCheckUsecase.Refine(c.Request.Context(), refineQuery, nil)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

			pubapi.
				NewResponse(dto.AdaptSanctionCheck(sanctionCheck)).
				WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId}).
				Serve(c)

		case false:
			sanctionCheck, err := sanctionCheckUsecase.Search(c.Request.Context(), refineQuery)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

			matchPayload := func(m models.SanctionCheckMatch) json.RawMessage {
				return m.Payload
			}

			pubapi.
				NewResponse(pure_utils.Map(sanctionCheck.Matches, matchPayload)).
				WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId}).
				Serve(c)
		}
	}
}

func HandleGetSanctionCheckEntity(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		entityId := c.Param("entityId")

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		entity, err := sanctionCheckUsecase.GetEntity(ctx, entityId)
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
		creds, _ := utils.CredentialsFromCtx(ctx)

		var params AddWhitelistParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		if err := sanctionCheckUsecase.CreateWhitelist(ctx, nil, creds.OrganizationId,
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
		creds, _ := utils.CredentialsFromCtx(ctx)

		var params DeleteWhitelistParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		if err := sanctionCheckUsecase.DeleteWhitelist(ctx, nil, creds.OrganizationId,
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
		creds, _ := utils.CredentialsFromCtx(ctx)

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
		sanctionCheckUsecase := uc.NewSanctionCheckUsecase()

		whitelists, err := sanctionCheckUsecase.SearchWhitelist(ctx, nil,
			creds.OrganizationId, params.Counterparty, params.EntityId, nil)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(pure_utils.Map(whitelists, dto.AdaptSanctionCheckWhitelist)).
			Serve(c)
	}
}
