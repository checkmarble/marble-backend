package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleDeleteDataModelTable(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		tableId := c.Param("tableID")
		dryRun := c.Query("perform") != "true"

		report, err := usecase.DeleteTable(ctx, dryRun, tableId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelDeleteFieldReport(report))
	}
}

func handleDeleteDataModelField(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		fieldId := c.Param("fieldID")
		dryRun := c.Query("perform") != "true"

		report, err := usecase.DeleteField(ctx, dryRun, fieldId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelDeleteFieldReport(report))
	}
}

func handleDeleteDataModelLink(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		linkId := c.Param("linkID")
		dryRun := c.Query("perform") != "true"

		report, err := usecase.DeleteLink(ctx, dryRun, linkId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelDeleteFieldReport(report))
	}
}

func handleDeleteDataModelPivot(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		pivotId := c.Param("pivotID")
		dryRun := c.Query("perform") != "true"

		report, err := usecase.DeletePivot(ctx, dryRun, pivotId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelDeleteFieldReport(report))
	}
}
