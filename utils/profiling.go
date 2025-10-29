package utils

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"

	"cloud.google.com/go/profiler"
	"github.com/gin-gonic/gin"
)

func SetupProfilerEndpoints(r *gin.Engine, serviceName, serviceVersion, gcpProjectId string) {
	switch key := os.Getenv("DEBUG_PROFILING_MODE"); key {
	case "gcp":
		cfg := profiler.Config{
			ProjectID:      gcpProjectId,
			Service:        serviceName,
			ServiceVersion: serviceVersion,
		}

		if err := profiler.Start(cfg); err != nil {
			fmt.Println(err)
		}

	case "http":
		pp := r.Group("/debug/pprof")
		pp.Use(func(c *gin.Context) {
			if c.Request.Header.Get("authorization") != "Bearer "+os.Getenv("DEBUG_PROFILING_TOKEN") {
				c.AbortWithStatus(http.StatusUnauthorized)
			}
		})

		pp.GET("/profile", gin.WrapF(pprof.Profile))
		pp.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		pp.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		pp.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
		pp.GET("/block", gin.WrapH(pprof.Handler("block")))
		pp.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	}
}
