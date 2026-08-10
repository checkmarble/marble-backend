package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
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

		opts := models.GraphWalkOptions{
			EndTypes: parseGraphEndTypes(c.Query("types")),
			Degrees:  parseGraphDegrees(c.Query("degrees")),
		}

		usecase := usecasesWithCreds(ctx, uc).NewGraphWalkUsecase()
		result, err := usecase.WalkGraph(ctx, organizationId, nodeType, nodeId, opts)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGraphResultDto(result))
	}
}

func parseGraphEndTypes(raw string) []string {
	var endTypes []string

	for part := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			endTypes = append(endTypes, trimmed)
		}
	}

	return endTypes
}

func parseGraphDegrees(raw string) int {
	if degrees, err := strconv.Atoi(raw); err == nil && degrees > 0 {
		return degrees
	}
	return 0
}
