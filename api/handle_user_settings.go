package api

import (
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

type UnavailabilityDto struct {
	Until time.Time `json:"until"`
}

func handleGetUnavailability(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewUserSettingsUsecase()

		avail, err := uc.GetUnavailability(ctx)

		if presentError(ctx, c, err) {
			return
		}
		if avail == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, UnavailabilityDto{Until: avail.UntilDate})
	}
}

func handleSetUnavailability(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input UnavailabilityDto

		if err := c.ShouldBindJSON(&input); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewUserSettingsUsecase()

		if err := uc.SetUnavailability(ctx, input.Until); presentError(ctx, c, err) {
			return
		}
	}
}

func handleDeleteUnavailability(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewUserSettingsUsecase()

		if err := uc.DeleteUnavailability(ctx); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
