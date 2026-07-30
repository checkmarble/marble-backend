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
			EndTypes: parseGraphEndTypes(c.Query("end_types")),
			Degrees:  parseGraphDegrees(c.Query("degrees"), c.Query("depth")),
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
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			endTypes = append(endTypes, trimmed)
		}
	}
	return endTypes
}

// parseGraphDegrees reads how many degrees to walk. Zero (missing, unparseable or not a
// positive number) lets the usecase apply its default; the usecase also owns the upper bound,
// so an over-large value is passed through and clamped there. `depth` is accepted as an alias
// so callers written against the earlier parameter name keep working.
func parseGraphDegrees(raw, alias string) int {
	if raw == "" {
		raw = alias
	}

	degrees, err := strconv.Atoi(raw)
	if err != nil || degrees <= 0 {
		return 0
	}

	return degrees
}
