package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleLivenessProbe(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		usecase := uc.NewLivenessUsecase()
		err := usecase.Liveness(c.Request.Context())
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mood": "Feu flammes !",
		})
	}
}
