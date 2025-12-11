package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleListAuditEvents(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters dto.AuditEventFilters

		if err := c.ShouldBindQuery(&filters); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		filters.OrgId = uuid.MustParse(orgId)

		if filters.Limit == 0 {
			filters.Limit = 10
		}

		uc := usecasesWithCreds(ctx, uc)
		auditUsecase := uc.NewAuditUsecase()

		events, err := auditUsecase.ListAuditEvents(ctx, filters)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.PaginatedAuditEvents{
			Events:      pure_utils.Map(events.Items, dto.AdaptAuditEvent),
			HasNextPage: events.HasNextPage,
		})
	}
}
