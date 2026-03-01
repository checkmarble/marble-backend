package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

		if err := c.ShouldBindJSON(&spec); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecase := uc.NewOrgImportUsecase()

		// If this endpoint is called by marble admin, the organizationId is Nil
		// and a new organization is created. Otherwise the organizationId corresponds
		// to the organization the admin is currently in, which will be filled.
		orgId, err := orgImportUsecase.Import(ctx, uc.Credentials.OrganizationId, spec, c.Query("seed") == "true")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"org_id": orgId,
		})
	}
}

func handleOrgImportFromFile(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var spec dto.OrgImport
		if err := json.Unmarshal(data, &spec); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := binding.Validator.ValidateStruct(&spec); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecase := uc.NewOrgImportUsecase()

		// If this endpoint is called by marble admin, the organizationId is Nil
		// and a new organization is created. Otherwise the organizationId corresponds
		// to the organization the admin is currently in, which will be filled.
		orgId, err := orgImportUsecase.Import(ctx, uc.Credentials.OrganizationId, spec, c.Query("seed") == "true")
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

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecase := uc.NewOrgImportUsecase()

		// If this endpoint is called by marble admin, the organizationId is Nil
		// Otherwise the organizationId is equal to the organization admin is currently in.
		orgId, err := orgImportUsecase.ImportFromArchetype(
			ctx,
			uc.Credentials.OrganizationId,
			apply,
			c.Query("seed") == "true",
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"org_id": orgId,
		})
	}
}

func handleOrgExport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, errors.Wrap(models.UnAuthorizedError, "credentials not found in context"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrgExportUsecase()

		result, err := usecase.Export(ctx, creds.OrganizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
