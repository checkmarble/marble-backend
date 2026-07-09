package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type whoamiInput struct{}

type whoamiOutput struct {
	Authenticated  bool   `json:"authenticated"`
	OrganizationId string `json:"organization_id,omitempty"`
	Role           string `json:"role,omitempty"`
}

// registerDebugTools registers a diagnostic tool useful for verifying which
// organization/role a given API key is authenticated as before running the
// other tools.
func registerDebugTools(server *sdkmcp.Server, _ usecases.Usecases) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "whoami",
		Description: "Debug tool: reports the organization and role the current MCP session is authenticated as.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in whoamiInput) (*sdkmcp.CallToolResult, whoamiOutput, error) {
		creds, ok := utils.CredentialsFromCtx(ctx)
		if !ok {
			return nil, whoamiOutput{Authenticated: false}, nil
		}
		return nil, whoamiOutput{
			Authenticated:  true,
			OrganizationId: creds.OrganizationId.String(),
			Role:           creds.Role.String(),
		}, nil
	})
}
