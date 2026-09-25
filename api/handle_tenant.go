package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleListTenants(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenants, err := usecasesWithCreds(ctx, uc).NewTenantUsecase().List(ctx)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"tenants": tenants})
	}
}

type tenantUpdateBody struct {
	Name string `json:"name" binding:"required"`
}

func handlePatchTenant(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := uuid.Parse(c.Param("tenant_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid tenant id"))
			return
		}
		var body tenantUpdateBody
		if err := c.ShouldBindJSON(&body); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if presentError(ctx, c, usecasesWithCreds(ctx, uc).NewTenantUsecase().UpdateName(ctx, tenantId, body.Name)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

type tenantMergeBody struct {
	SourceTenantIds []uuid.UUID `json:"source_tenant_ids" binding:"required,min=1"`
	Name            *string     `json:"name"`
}

func handlePostTenantMerge(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		targetTenantId, err := uuid.Parse(c.Param("tenant_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid tenant id"))
			return
		}
		var body tenantMergeBody
		if err := c.ShouldBindJSON(&body); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		err = usecasesWithCreds(ctx, uc).NewTenantUsecase().Merge(ctx, usecases.TenantMergeInput{
			TargetTenantId:  targetTenantId,
			SourceTenantIds: body.SourceTenantIds,
			NewName:         body.Name,
		})
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
