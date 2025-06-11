package pubapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	timeout "github.com/vearne/gin-timeout"
)

func TimeoutMiddleware(duration time.Duration) gin.HandlerFunc {
	return timeout.Timeout(
		timeout.WithTimeout(duration),
		timeout.WithErrorHttpCode(http.StatusRequestTimeout),
		timeout.WithDefaultMsg("Request timeout"),
	)
}
