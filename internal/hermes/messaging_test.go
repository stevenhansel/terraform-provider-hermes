package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReadsAndUpdatesMessagingPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/messaging/platforms":
			if request.Method != http.MethodGet {
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if got := request.URL.Query().Get("profile"); got != "assistant" {
				t.Errorf("messaging profile = %q, want assistant", got)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{
                "env_path": "/redacted/.env",
                "platforms": [{
                    "id": "homeassistant", "name": "Home Assistant", "description": "Control the home",
                    "docs_url": "https://www.home-assistant.io/docs/authentication/", "enabled": true,
                    "configured": true, "gateway_running": false, "state": "gateway_stopped",
                    "error_code": null, "error_message": "must not be persisted",
                    "updated_at": "2026-09-12T00:00:00Z",
                    "env_vars": [
                        {"key": "HASS_URL", "required": true, "is_set": true, "is_password": false},
                        {"key": "HASS_TOKEN", "required": true, "is_set": true, "is_password": true, "redacted_value": "super-secret"}
                    ]
                }]
            }`))
		case "/api/messaging/platforms/homeassistant":
			if request.Method != http.MethodPut {
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var body MessagingPlatformUpdateRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode messaging update: %v", err)
			}
			if body.Enabled == nil || !*body.Enabled || body.Env["HASS_TOKEN"] != "token-from-secret-store" || len(body.ClearEnv) != 1 || body.ClearEnv[0] != "HASS_URL" {
				t.Errorf("messaging update = %#v, want endpoint update without response secrets", body)
			}
			if got := request.URL.Query().Get("profile"); got != "assistant" {
				t.Errorf("update profile = %q, want assistant", got)
			}
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
	platforms, err := client.ListMessagingPlatforms(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ListMessagingPlatforms: %v", err)
	}
	if len(platforms) != 1 || platforms[0].ID != "homeassistant" || !platforms[0].Configured || len(platforms[0].EnvVars) != 2 {
		t.Fatalf("platforms = %#v, want one decoded Home Assistant platform", platforms)
	}
	if platforms[0].EnvVars[1].IsPassword != true {
		t.Fatalf("password metadata = %#v, want password marker", platforms[0].EnvVars[1])
	}

	enabled := true
	err = client.UpdateMessagingPlatform(context.Background(), "assistant", "homeassistant", MessagingPlatformUpdateRequest{
		Enabled: &enabled,
		Env:     map[string]string{"HASS_TOKEN": "token-from-secret-store"}, ClearEnv: []string{"HASS_URL"},
	})
	if err != nil {
		t.Fatalf("UpdateMessagingPlatform: %v", err)
	}
}
