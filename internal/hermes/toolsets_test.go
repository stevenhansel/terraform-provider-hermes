package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReadsAndTogglesToolset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/auth/password-login" {
			response.WriteHeader(http.StatusOK)
			return
		}
		if request.URL.Query().Get("profile") != "assistant" {
			t.Errorf("profile query = %q, want assistant", request.URL.Query().Get("profile"))
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/tools/toolsets":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`[{"name":"web","label":"Web","description":"Browser tools","platform":"cli","platform_label":"CLI","enabled":true,"available":true,"configured":true,"tools":["browser"]}]`))
		case request.Method == http.MethodPut && request.URL.Path == "/api/tools/toolsets/web":
			var body toolsetToggleRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode toolset toggle: %v", err)
			}
			if body.Enabled || body.Profile != "assistant" {
				t.Errorf("toolset toggle = %#v, want disabled assistant toolset", body)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"name":"web","platform":"cli","enabled":false,"post_setup_started":null}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	toolsets, err := client.ListToolsets(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ListToolsets: %v", err)
	}
	if len(toolsets) != 1 || toolsets[0].Name != "web" || len(toolsets[0].Tools) != 1 || !toolsets[0].Configured {
		t.Fatalf("toolsets = %#v, want decoded web toolset", toolsets)
	}
	result, err := client.SetToolsetEnabled(context.Background(), "assistant", "web", false)
	if err != nil {
		t.Fatalf("SetToolsetEnabled: %v", err)
	}
	if !result.OK || result.Enabled || result.PostSetupStarted != nil {
		t.Fatalf("toggle result = %#v, want successful disabled result", result)
	}
}
