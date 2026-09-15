package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleListScreeningManualSearch(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc)
		searchesUc := uc.NewScreeningSavedSearchesUsecase()

		searches, err := searchesUc.ListSearches(ctx)
		if presentError(ctx, c, err) {
			return
		}

		out, err := pure_utils.MapErr(searches, func(search models.ScreeningSavedSearch) (dto.ScreeningSavedSearch, error) {
			return dto.AdaptScreeningSavedSearch(search)
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleSaveScreeningManualSearch(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.CreateScreeningSavedSearch

		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		searchesUc := uc.NewScreeningSavedSearchesUsecase()

		search, err := searchesUc.SaveSearch(ctx, payload.Name, payload.Config)
		if presentError(ctx, c, err) {
			return
		}

		out, err := dto.AdaptScreeningSavedSearch(search)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, out)
	}
}

func handleDeleteScreeningSavedSearch(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		searchId, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		searchesUc := uc.NewScreeningSavedSearchesUsecase()

		if err := searchesUc.DeleteSavedSearch(ctx, searchId); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
