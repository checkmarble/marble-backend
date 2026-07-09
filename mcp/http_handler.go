package mcp

import (
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/usecases"
)

// NewHTTPHandler wraps the MCP server in a streamable-HTTP http.Handler,
// mountable behind Gin via gin.WrapH.
func NewHTTPHandler(uc usecases.Usecases, version string) http.Handler {
	server := NewServer(uc, version)

	return sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, nil)
}
