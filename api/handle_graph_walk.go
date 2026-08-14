package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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

		relationGroupIds, err := parseGraphRelationGroups(c.Query("same_field_relations"))
		if presentError(ctx, c, err) {
			return
		}

		opts := models.GraphWalkOptions{
			EndTypes:               parseGraphEndTypes(c.Query("types")),
			Degrees:                parseGraphDegrees(c.Query("degrees")),
			SkipSameFieldRelations: c.Query("skip_same_field_relations") == "true",
			SameFieldRelations:     relationGroupIds,
		}

		usecase := usecasesWithCreds(ctx, uc).NewGraphWalkUsecase()
		result, err := usecase.WalkGraph(ctx, organizationId, nodeType, nodeId, opts)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGraphResultDto(result))
	}
}

// parseGraphRelationGroups reads the relation groups a walk should restrict its same-field
// traversal to, as a comma-separated list of group ids. Empty means "every group".
func parseGraphRelationGroups(raw string) ([]uuid.UUID, error) {
	var groupIds []uuid.UUID

	for part := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		groupId, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, errors.Wrapf(models.BadParameterError,
				"%q is not a valid relation group id", trimmed)
		}

		groupIds = append(groupIds, groupId)
	}

	return groupIds, nil
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
