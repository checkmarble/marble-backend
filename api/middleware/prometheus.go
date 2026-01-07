package middleware

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func PrometheusMiddleware(c *gin.Context) {
	start := time.Now()

	c.Next()

	orgId, _ := utils.OrganizationIdFromRequest(c.Request)
	orgIdString := orgId.String()

	utils.MetricRequestCount.
		With(prometheus.Labels{"org_id": orgIdString, "method": c.Request.Method, "url": c.FullPath(), "status": fmt.Sprintf("%d", c.Writer.Status())}).
		Inc()

	utils.MetricRequestLatency.
		With(prometheus.Labels{"org_id": orgIdString, "method": c.Request.Method, "url": c.FullPath(), "status": fmt.Sprintf("%d", c.Writer.Status())}).
		Observe(time.Since(start).Seconds())
}
