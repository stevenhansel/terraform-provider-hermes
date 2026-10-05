package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientManagesAgentPlugin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/auth/password-login" {
			response.WriteHeader(http.StatusOK)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/dashboard/plugins/hub":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"plugins":[{"name":"calendar","version":"1.2.3","description":"Calendar tools","source":"user","runtime_status":"enabled","has_dashboard_manifest":false,"can_remove":true,"can_update_git":true,"auth_required":false,"user_hidden":false}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/dashboard/plugins/catalog":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"entries":[{"name":"calendar","repo":"example/calendar","sha":"abcdef","installed":true,"installed_sha":"abcdef","runtime_status":"enabled"}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/dashboard/agent-plugins/install":
			var body AgentPluginInstallRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode plugin install: %v", err)
			}
			if body.Identifier != "example/calendar" || !body.Enable || body.Force || body.Ref != "" {
				t.Errorf("plugin install = %#v, want reviewed default install", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"plugin_name":"calendar","warnings":["reviewed source"],"missing_env":["CALENDAR_URL"],"enabled":true}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/dashboard/agent-plugins/calendar/enable":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"name":"calendar","unchanged":false}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/dashboard/agent-plugins/calendar/disable":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"name":"calendar","unchanged":false}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/api/dashboard/agent-plugins/calendar":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"name":"calendar"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := client.ListAgentPlugins(context.Background())
	if err != nil {
		t.Fatalf("ListAgentPlugins: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "calendar" || !plugins[0].CanRemove {
		t.Fatalf("plugins = %#v, want calendar plugin", plugins)
	}
	catalog, err := client.ListPluginCatalog(context.Background())
	if err != nil || len(catalog) != 1 || catalog[0].InstalledSHA != "abcdef" {
		t.Fatalf("ListPluginCatalog = %#v, %v", catalog, err)
	}
	result, err := client.InstallAgentPlugin(context.Background(), AgentPluginInstallRequest{Identifier: "example/calendar", Enable: true})
	if err != nil || !result.OK || result.PluginName != "calendar" || len(result.MissingEnv) != 1 {
		t.Fatalf("InstallAgentPlugin = %#v, %v", result, err)
	}
	if _, err := client.SetAgentPluginEnabled(context.Background(), "calendar", true); err != nil {
		t.Fatalf("SetAgentPluginEnabled(true): %v", err)
	}
	if _, err := client.SetAgentPluginEnabled(context.Background(), "calendar", false); err != nil {
		t.Fatalf("SetAgentPluginEnabled(false): %v", err)
	}
	if _, err := client.DeleteAgentPlugin(context.Background(), "calendar"); err != nil {
		t.Fatalf("DeleteAgentPlugin: %v", err)
	}
}
