package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
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

		var spec dto.OrgImport

		if err := c.ShouldBindJSON(&spec); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		orgImportUsecse := uc.NewOrgImportUsecase()

		if err := orgImportUsecse.Seed(ctx, spec, "019b087e-a3d6-7f1f-bbd7-5903c2e373da"); presentError(ctx, c, err) {
			return
		}
	}
}
