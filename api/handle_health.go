package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleHealth(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := uc.NewHealthUsecase()
		status := usecase.GetHealthStatus(ctx)

		c.JSON(http.StatusOK, dto.AdaptHealthStatus(status))
	}
}
