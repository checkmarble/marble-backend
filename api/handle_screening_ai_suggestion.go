package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleEnqueueScreeningHitSuggestion(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		screeningId := c.Param("screeningId")

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		err := usecase.EnqueueScreeningHitSuggestion(ctx, screeningId)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusAccepted)
	}
}

func handleGetScreeningSuggestions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		screeningId := c.Param("screeningId")

		usecase := usecasesWithCreds(ctx, uc).NewAiAgentUsecase()
		suggestions, err := usecase.GetScreeningSuggestions(ctx, screeningId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, suggestions)
	}
}
