package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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

		switch c.FullPath() {
		case "/admin/audit-events/download":
			if filters.Limit > 10000 {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "maximum page size is 10000"))
				return
			}

			events, err := auditUsecase.DownloadAuditEvents(ctx, filters)
			if presentError(ctx, c, err) {
				return
			}

			c.Header("content-type", "application/jsonl")

			enc := json.NewEncoder(c.Writer)

			c.Stream(func(w io.Writer) bool {
				for event := range events.Models {
					if presentError(ctx, c, event.Error) {
						return false
					}
					if err := enc.Encode(event.Model); presentError(ctx, c, err) {
						return false
					}
					w.(http.Flusher).Flush()
				}

				return false
			})

		default:
			if filters.Limit > 100 {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "maximum page size is 100"))
				return
			}

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
}
