package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientManagesProfilesAndReadsStatus(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.RequestURI())
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/profiles":
			switch request.Method {
			case http.MethodGet:
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(`{"profiles":[{"name":"assistant","path":"/redacted","is_default":false,"description":"Personal helper","skill_count":3,"gateway_running":true}]}`))
			case http.MethodPost:
				var body CreateProfileRequest
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode create profile: %v", err)
				}
				if body.Name != "assistant" || body.CloneFrom != "default" || !body.NoSkills {
					t.Errorf("unexpected create body: %#v", body)
				}
				response.WriteHeader(http.StatusOK)
			}
		case "/api/profiles/assistant":
			if request.Method == http.MethodPatch {
				var body struct {
					NewName string `json:"new_name"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode rename profile: %v", err)
				}
				if body.NewName != "helper" {
					t.Errorf("new profile name = %q, want helper", body.NewName)
				}
			}
			response.WriteHeader(http.StatusOK)
		case "/api/profiles/assistant/description":
			if request.Method != http.MethodPut {
				t.Errorf("description method = %s, want PUT", request.Method)
			}
			response.WriteHeader(http.StatusOK)
		case "/api/profiles/helper":
			if request.Method != http.MethodDelete {
				t.Errorf("delete method = %s, want DELETE", request.Method)
			}
			response.WriteHeader(http.StatusOK)
		case "/api/status":
			if got := request.URL.Query().Get("profile"); got != "assistant" {
				t.Errorf("status profile = %q, want assistant", got)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"version":"0.21.0","release_date":"2026-01-01","gateway_running":true,"gateway_state":"running","active_agents":1,"active_sessions":2,"gateway_busy":false,"gateway_drainable":true,"auth_required":true,"auth_providers":["basic","oidc"],"auth_flows":["cookie","native_pkce"],"overall":"ok"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := client.ListProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Name != "assistant" || profiles[0].SkillCount != 3 {
		t.Fatalf("profiles = %#v, want assistant profile", profiles)
	}

	if err := client.CreateProfile(context.Background(), CreateProfileRequest{
		Name: "assistant", CloneFrom: "default", NoSkills: true,
	}); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := client.RenameProfile(context.Background(), "assistant", "helper"); err != nil {
		t.Fatalf("rename profile: %v", err)
	}
	if err := client.UpdateProfileDescription(context.Background(), "assistant", "Updated"); err != nil {
		t.Fatalf("update profile description: %v", err)
	}
	if err := client.DeleteProfile(context.Background(), "helper"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	status, err := client.GetStatus(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.Version != "0.21.0" || !status.GatewayRunning || status.ActiveSessions != 2 {
		t.Fatalf("status = %#v, want decoded Hermes status", status)
	}
	if len(paths) != 7 {
		t.Fatalf("request count = %d (%v), want login plus six API calls", len(paths), paths)
	}
}

func TestClientReadsModelOptionsWithProfileAndFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/model/options":
			query := request.URL.Query()
			for key, want := range map[string]string{
				"profile":              "assistant",
				"refresh":              "true",
				"include_unconfigured": "true",
				"explicit_only":        "false",
			} {
				if got := query.Get(key); got != want {
					t.Errorf("%s query = %q, want %q", key, got, want)
				}
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{
                "providers": [{
                    "slug": "llamacpp", "name": "llama.cpp", "models": ["local-model"],
                    "total_models": 1, "is_current": true, "is_user_defined": true,
                    "source": "custom", "authenticated": true, "auth_type": "none",
                    "key_env": "", "warning": ""
                }],
                "model": "local-model", "provider": "llamacpp"
            }`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	options, err := client.GetModelOptions(context.Background(), "assistant", true, true, false)
	if err != nil {
		t.Fatalf("GetModelOptions: %v", err)
	}
	if options.Provider != "llamacpp" || options.Model != "local-model" || len(options.Providers) != 1 {
		t.Fatalf("options = %#v, want one local provider", options)
	}
	if options.Providers[0].Slug != "llamacpp" || !options.Providers[0].IsCurrent || len(options.Providers[0].Models) != 1 {
		t.Fatalf("provider option = %#v, want decoded model row", options.Providers[0])
	}
}

func TestHTTPErrorRetainsStatusWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/profiles":
			response.WriteHeader(http.StatusNotFound)
			_, _ = response.Write([]byte("secret response body"))
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListProfiles(context.Background())
	if err == nil || !IsNotFound(err) {
		t.Fatalf("ListProfiles error = %v, want not-found HTTPError", err)
	}
	if got := err.Error(); got != "hermes request GET /api/profiles failed with HTTP 404" {
		t.Fatalf("error = %q, want status-only error", got)
	}
}
