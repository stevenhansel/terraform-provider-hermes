package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// MCPServer is the dashboard summary returned for a configured MCP server.
// Config contains the server's non-secret fields as returned by Hermes. The
// provider intentionally does not expose bearer tokens or environment values
// from this response.
type MCPServer struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Auth      string   `json:"auth"`
	Tools     []string `json:"tools"`
	Enabled   bool     `json:"enabled"`
}

type mcpListResponse struct {
	Servers []MCPServer `json:"servers"`
}

// MCPServerRequest is the HTTP/SSE registration contract. Stdio command
// execution is intentionally not represented by this resource because it
// creates a host-process execution boundary.
type MCPServerRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Auth        string `json:"auth,omitempty"`
	BearerToken string `json:"bearer_token,omitempty"`
	Profile     string `json:"profile,omitempty"`
}

// ListMCPServers returns the configured server summaries for a profile.
func (c *Client) ListMCPServers(ctx context.Context, profile string) ([]MCPServer, error) {
	var result mcpListResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/api/mcp/servers", profileQuery(profile), nil, &result); err != nil {
		return nil, err
	}
	return result.Servers, nil
}

// CreateMCPServer registers an HTTP/SSE MCP server.
func (c *Client) CreateMCPServer(ctx context.Context, request MCPServerRequest) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodPost, "/api/mcp/servers", profileQuery(request.Profile), request, nil)
}

// ReplaceMCPServers replaces Hermes' complete server map. It is deliberately
// not used by the singleton MCP resource: callers must provide the complete
// map because Hermes treats this endpoint as authoritative and removes every
// omitted server.
func (c *Client) ReplaceMCPServers(ctx context.Context, profile string, servers map[string]map[string]any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	body := struct {
		Servers map[string]map[string]any `json:"servers"`
		Profile string                    `json:"profile,omitempty"`
	}{Servers: servers, Profile: profile}
	return c.requestJSON(ctx, http.MethodPut, "/api/mcp/servers", profileQuery(profile), body, nil)
}

// DeleteMCPServer removes one MCP server from a profile.
func (c *Client) DeleteMCPServer(ctx context.Context, profile, name string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodDelete, "/api/mcp/servers/"+url.PathEscape(name), profileQuery(profile), nil, nil)
}

// SetMCPServerEnabled changes only the enabled flag and preserves the server
// registration itself.
func (c *Client) SetMCPServerEnabled(ctx context.Context, profile, name string, enabled bool) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	body := struct {
		Enabled bool   `json:"enabled"`
		Profile string `json:"profile,omitempty"`
	}{Enabled: enabled, Profile: profile}
	path := "/api/mcp/servers/" + url.PathEscape(name) + "/enabled"
	return c.requestJSON(ctx, http.MethodPut, path, profileQuery(profile), body, nil)
}
