package v2

import (
	"net/http"

	v1 "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, authF gin.HandlerFunc, uc usecases.Usecases) {
	r.GET("/-/health", version)

	{
		r := r.Group("/", authF)

		r.GET("/decisions/:decisionId/sanction-checks", v1.HandleListSanctionChecks(uc))
	}
}

func version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": "v2"})
}
