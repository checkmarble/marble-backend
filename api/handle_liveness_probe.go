package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleLivenessProbe(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := uc.NewLivenessUsecase()
		status := usecase.Liveness(ctx)

		if !status.IsLive() {
			// Keep the same status code as before, need to check if we can use a different one
			c.JSON(http.StatusInternalServerError, dto.AdaptLivenessStatus(status))
			return
		}

		c.JSON(http.StatusOK, dto.AdaptLivenessStatus(status))
	}
}
