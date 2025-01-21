package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListSanctionChecks(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionId := c.Query("decision_id")

		if decisionId == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewSanctionCheckUsecase()
		sanctionChecks, err := uc.ListSanctionChecks(ctx, decisionId)

		if presentError(ctx, c, err) {
			return
		}

		sanctionCheckJson := pure_utils.Map(sanctionChecks, dto.AdaptSanctionCheckDto)

		c.JSON(http.StatusOK, sanctionCheckJson)
	}
}
