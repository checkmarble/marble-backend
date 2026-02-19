package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleListArchetypes(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc).NewOrgImportUsecase()
		archetypes, err := uc.ListArchetypes(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptArchetypesDto(archetypes))
	}
}

func handleOrgImport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var spec dto.OrgImport

		err := c.ShouldBindJSON(&spec)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var existingOrgId *uuid.UUID
		if creds, found := utils.CredentialsFromCtx(ctx); found && creds.OrganizationId != uuid.Nil {
			existingOrgId = &creds.OrganizationId
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecse := uc.NewOrgImportUsecase()

		orgId, err := orgImportUsecse.Import(ctx, existingOrgId, spec, c.Query("seed") == "true")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"org_id": orgId,
		})
	}
}

func handleOrgImportFromArchetype(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var apply dto.ArchetypeApplyDto

		err := c.ShouldBindJSON(&apply)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var existingOrgId *uuid.UUID
		if creds, found := utils.CredentialsFromCtx(ctx); found && creds.OrganizationId != uuid.Nil {
			existingOrgId = &creds.OrganizationId
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecase := uc.NewOrgImportUsecase()

		orgId, err := orgImportUsecase.ImportFromArchetype(ctx, existingOrgId, apply, c.Query("seed") == "true")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"org_id": orgId,
		})
	}
}
