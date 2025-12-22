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

		report, err := usecase.DeleteTable(ctx, tableId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.Status(http.StatusNoContent)
	}
}

func handleDeleteDataModelField(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		fieldId := c.Param("fieldID")

		report, err := usecase.DeleteField(ctx, fieldId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.Status(http.StatusNoContent)
	}
}

func handleDeleteDataModelLink(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewDataModelDestroyUsecase()
		linkId := c.Param("linkID")

		report, err := usecase.DeleteLink(ctx, linkId)
		if err != nil {
			if errors.Is(err, models.ConflictError) {
				c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report))
				return
			}
			if presentError(ctx, c, err) {
				return
			}
		}

		c.Status(http.StatusNoContent)
	}
}
