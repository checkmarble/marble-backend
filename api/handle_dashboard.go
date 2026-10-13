package api

import (
	"net/http"
	"strconv"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleDashboard(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		months, err := strconv.Atoi(c.DefaultQuery("months", "6"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "months must be 1, 3, 6, or 12"))
			return
		}

		dashboard, err := usecasesWithCreds(ctx, uc).NewDashboardUsecase().Get(ctx, months)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDashboard(dashboard))
	}
}
