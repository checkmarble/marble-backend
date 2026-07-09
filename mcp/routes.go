package mcp

import (
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/usecases"
)

// Routes mounts the MCP streamable-HTTP endpoint on group, behind
// authMiddleware. It mirrors the pattern used for the public API v1 group
// in api/api/routes.go, reusing the same Gin authentication middleware.
func Routes(uc usecases.Usecases, version string, group *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	handler := NewHTTPHandler(uc, version)

	group.Use(authMiddleware)
	group.Any("", gin.WrapH(handler))
}
