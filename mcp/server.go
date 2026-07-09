package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/usecases"
)

// NewServer builds the MCP server exposing Marble reporting/action tools.
//
// A single *sdkmcp.Server instance is shared across all MCP sessions; each
// tool handler pulls the caller's org-scoped credentials from ctx (populated
// by the Gin auth middleware that fronts this server, see mcp/routes.go),
// so no per-session/per-credential server instances are needed.
func NewServer(uc usecases.Usecases, version string) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "marble-backend",
		Version: version,
	}, nil)

	registerDebugTools(server, uc)
	registerReportingTools(server, uc)
	registerEntityTools(server, uc)
	registerActionTools(server, uc)

	return server
}
