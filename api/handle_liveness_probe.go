package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleLivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"mood": "Feu flammes !",
	})
}
