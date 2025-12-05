package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func HandleCreateContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var param params.CreateContinuousScreeningObjectParams
		if err := c.ShouldBindJSON(&param); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if err := param.Validate(); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreening, err := uc.CreateContinuousScreeningObject(
			ctx,
			param.ToModel(),
		)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.NewResponse(dto.AdaptContinuousScreening(continuousScreening)).Serve(c, http.StatusCreated)
	}
}

func HandleDeleteContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var param params.DeleteContinuousScreeningObjectParams
		if err := c.ShouldBindJSON(&param); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		err := uc.DeleteContinuousScreeningObject(
			ctx,
			param.ToModel(),
		)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
