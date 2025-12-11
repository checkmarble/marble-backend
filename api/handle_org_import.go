package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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

func handleOrgSeed(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := uuid.Parse(c.Param("orgId"))
		if presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		var spec dto.OrgImport

		if err := c.ShouldBindJSON(&spec); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecse := uc.NewOrgImportUsecase()

		if err := orgImportUsecse.Seed(ctx, spec, orgId); presentError(ctx, c, err) {
			return
		}
	}
}
