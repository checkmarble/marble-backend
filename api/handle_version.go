package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleVersion(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		uc := uc.NewVersionUsecase()
		apiVersion := uc.GetApiVersion()

		c.JSON(http.StatusOK, gin.H{
			"version": apiVersion,
		})
	}
}
