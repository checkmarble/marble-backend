package v1

import (
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func HandleListSanctionChecks(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		decisionId, err := pubapi.UuidParam(c, "decisionId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(pubapi.ErrInvalidId).Serve(c)
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

		pubapi.
			NewResponse(pure_utils.Map(sc, dto.AdaptSanctionCheck)).
			WithLink(pubapi.LinkDecisions, gin.H{"id": decisionId.String()}).
			Serve(c)
	}
}

type UpdateSanctionCheckMatchStatusParams struct {
	Status string `json:"status" binding:"required,oneof=no_hit confirmed_hit"`
	Name   string `json:"name" binding:"required"`
}

func HandleUpdateSanctionCheckMatchStatus(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		matchId, err := pubapi.UuidParam(c, "matchId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(pubapi.ErrInvalidId).Serve(c)
			return
		}

		var params UpdateSanctionCheckMatchStatusParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(matchId).Serve(c)
	}
}
