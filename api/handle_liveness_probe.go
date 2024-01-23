package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleLivenessProbe(c *gin.Context) {
	fmt.Println("Liveness probe ok")
	c.JSON(http.StatusOK, gin.H{
		"mood": "Feu flammes !",
	})
}
