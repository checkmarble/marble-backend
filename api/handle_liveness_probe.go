package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleLivenessProbe(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := uc.NewLivenessUsecase()
		err := usecase.Liveness(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mood": "Feu flammes !",
		})
	}
}
