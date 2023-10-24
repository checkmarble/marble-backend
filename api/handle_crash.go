package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleCrash(c *gin.Context) {
	c.Status(http.StatusInternalServerError)
}
