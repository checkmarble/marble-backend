package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleGraphWalk(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		nodeType := c.Param("node_type")
		nodeId := c.Param("node_id")

		usecase := usecasesWithCreds(ctx, uc).NewGraphWalkUsecase()
		result, err := usecase.WalkGraph(ctx, organizationId, nodeType, nodeId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGraphResultDto(result))
	}
}
