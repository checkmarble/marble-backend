package v1beta

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func HandleGetObjectRiskLevel(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		record := models.ScoringRecordRef{
			OrgId:      orgId,
			RecordType: c.Param("objectType"),
			RecordId:   c.Param("objectId"),
		}

		opts := models.RefreshScoreOptions{
			RefreshOlderThan:    time.Hour,
			RefreshInBackground: true,
		}

		score, _, err := scoringUsecase.GetActiveScore(ctx, record, false, opts)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if score == nil {
			types.NewErrorResponse().Serve(c, http.StatusNotFound)
			return
		}

		overridenBy, err := userOrApiKey(ctx, uc, score.OverriddenBy)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptRiskLevel(*score, overridenBy)).
			Serve(c)
	}
}

func HandleOverrideObjectRiskLevel(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p params.RiskLevelOverrideParams

		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		req := models.InsertScoreRequest{
			OrgId:      orgId,
			RecordType: c.Param("objectType"),
			RecordId:   c.Param("objectId"),
			RiskLevel:  p.RiskLevel,
			Source:     models.ScoreSourceOverride,
		}

		score, err := scoringUsecase.OverrideScore(ctx, req)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		overridenBy, err := userOrApiKey(ctx, uc, score.OverriddenBy)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptRiskLevel(score, overridenBy)).
			Serve(c, http.StatusCreated)
	}
}

func userOrApiKey(ctx context.Context, uc *usecases.UsecasesWithCreds, id *uuid.UUID) (*dto.Ref, error) {
	if id == nil {
		return nil, nil
	}

	userUsecase := uc.NewUserUseCase()

	user, err := userUsecase.GetUser(ctx, id.String())
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return nil, err
	}
	if err == nil {
		return new(dto.AdaptUserRef(user)), nil
	}

	return &dto.Ref{Id: id.String(), Name: "api_key"}, nil
}
