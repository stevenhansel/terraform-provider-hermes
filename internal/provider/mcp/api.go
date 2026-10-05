package mcp

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

// mcpClient is intentionally limited to the profile-scoped HTTP MCP
// operations supported by the resource. Whole-map replacement is not part of
// this interface because a singleton Terraform resource must not delete
// servers it does not own.
type mcpClient interface {
	ListMCPServers(context.Context, string) ([]hermes.MCPServer, error)
	CreateMCPServer(context.Context, hermes.MCPServerRequest) error
	DeleteMCPServer(context.Context, string, string) error
	SetMCPServerEnabled(context.Context, string, string, bool) error
}
