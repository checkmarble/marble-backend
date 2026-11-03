package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleSetTenantAdministrator(uc usecases.Usecases, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID, err := uuid.Parse(c.Param("tenant_id"))
		if err != nil {
			presentError(ctx, c, models.BadParameterError)
			return
		}
		userID, err := uuid.Parse(c.Param("user_id"))
		if err != nil {
			presentError(ctx, c, models.BadParameterError)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		if presentError(ctx, c, usecase.SetTenantAdministrator(ctx, tenantID, userID.String(), enabled)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
