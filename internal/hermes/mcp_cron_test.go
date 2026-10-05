package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientManagesMCPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/password-login" && request.URL.Query().Get("profile") != "assistant" {
			t.Errorf("profile query = %q, want assistant", request.URL.Query().Get("profile"))
		}
		switch {
		case request.URL.Path == "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/api/mcp/servers":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"servers":[{"name":"docs","transport":"http","url":"http://docs/mcp","auth":null,"tools":["search"],"enabled":true}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/mcp/servers":
			var body MCPServerRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode MCP create request: %v", err)
			}
			if body.Name != "docs" || body.URL != "http://docs/mcp" || body.Auth != "none" || body.BearerToken != "" || body.Profile != "assistant" {
				t.Errorf("MCP create request = %#v", body)
			}
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPut && request.URL.Path == "/api/mcp/servers/docs/enabled":
			var body struct {
				Enabled bool   `json:"enabled"`
				Profile string `json:"profile"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode MCP enabled request: %v", err)
			}
			if body.Enabled || body.Profile != "assistant" {
				t.Errorf("MCP enabled request = %#v, want disabled assistant server", body)
			}
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/mcp/servers/docs":
			response.WriteHeader(http.StatusOK)
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}

	servers, err := client.ListMCPServers(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	if len(servers) != 1 || servers[0].Name != "docs" || servers[0].Transport != "http" || servers[0].Auth != "" || !servers[0].Enabled {
		t.Fatalf("MCP servers = %#v", servers)
	}
	if err := client.CreateMCPServer(context.Background(), MCPServerRequest{
		Name: "docs", URL: "http://docs/mcp", Auth: "none", Profile: "assistant",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if err := client.SetMCPServerEnabled(context.Background(), "assistant", "docs", false); err != nil {
		t.Fatalf("SetMCPServerEnabled: %v", err)
	}
	if err := client.DeleteMCPServer(context.Background(), "assistant", "docs"); err != nil {
		t.Fatalf("DeleteMCPServer: %v", err)
	}
}
