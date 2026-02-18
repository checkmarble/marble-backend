package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListArchetypes(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		archetypes, err := uc.NewOrgImportUsecase().ListArchetypes(ctx)
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

		if err := c.ShouldBindJSON(&spec); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecse := uc.NewOrgImportUsecase()

		orgId, err := orgImportUsecse.Import(ctx, spec, c.Query("seed") == "true")
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

		var spec dto.OrgImport

		if err := c.ShouldBindJSON(&spec); presentError(ctx, c, err) {
			return
		}

		archetype := c.Param("archetype")
		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecse := uc.NewOrgImportUsecase()

		orgId, err := orgImportUsecse.ImportFromArchetype(ctx, archetype, spec, c.Query("seed") == "true")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"org_id": orgId,
		})
	}
}
