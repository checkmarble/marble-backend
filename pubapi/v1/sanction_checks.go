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

		var sanctionCheck models.SanctionCheckWithMatches

		switch write {
		case true:
			sanctionCheck, err = sanctionCheckUsecase.Refine(c.Request.Context(), refineQuery, nil)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}

		case false:
			sanctionCheck, err = sanctionCheckUsecase.Search(c.Request.Context(), refineQuery)
			if err != nil {
				pubapi.NewErrorResponse().WithError(err).Serve(c)
				return
			}
		}

		pubapi.
			NewResponse(dto.AdaptSanctionCheck(sanctionCheck)).
			WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId}).
			Serve(c)
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
