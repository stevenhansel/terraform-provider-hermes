package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestClientManagesCronJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/password-login" && request.URL.Query().Get("profile") != "assistant" {
			t.Errorf("profile query = %q, want assistant", request.URL.Query().Get("profile"))
		}
		switch {
		case request.URL.Path == "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && request.URL.Path == "/api/cron/jobs":
			var body CronJobRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode cron create request: %v", err)
			}
			wantSkills := []string{"morning-brief"}
			if body.Schedule != "every 1h" || body.Prompt != "Check my tasks" || !reflect.DeepEqual(body.Skills, wantSkills) || body.Provider != "custom" {
				t.Errorf("cron create request = %#v", body)
			}
			writeCronJob(response, false)
		case request.Method == http.MethodGet && request.URL.Path == "/api/cron/jobs/job123":
			writeCronJob(response, false)
		case request.Method == http.MethodPut && request.URL.Path == "/api/cron/jobs/job123":
			var body struct {
				Updates map[string]any `json:"updates"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode cron update request: %v", err)
			}
			if body.Updates["prompt"] != "Updated prompt" {
				t.Errorf("cron updates = %#v, want updated prompt", body.Updates)
			}
			writeCronJob(response, false)
		case request.Method == http.MethodPost && request.URL.Path == "/api/cron/jobs/job123/pause":
			writeCronJob(response, true)
		case request.Method == http.MethodPost && request.URL.Path == "/api/cron/jobs/job123/resume":
			writeCronJob(response, false)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/cron/jobs/job123":
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
	created, err := client.CreateCronJob(context.Background(), "assistant", CronJobRequest{
		Prompt: "Check my tasks", Schedule: "every 1h", Skills: []string{"morning-brief"}, Provider: "custom",
	})
	if err != nil {
		t.Fatalf("CreateCronJob: %v", err)
	}
	if created.ID != "job123" || created.ScheduleText() != "every 1h" || created.DeliverText() != "local" {
		t.Fatalf("created cron job = %#v", created)
	}
	if _, err := client.GetCronJob(context.Background(), "assistant", "job123"); err != nil {
		t.Fatalf("GetCronJob: %v", err)
	}
	if _, err := client.UpdateCronJob(context.Background(), "assistant", "job123", map[string]any{"prompt": "Updated prompt"}); err != nil {
		t.Fatalf("UpdateCronJob: %v", err)
	}
	paused, err := client.PauseCronJob(context.Background(), "assistant", "job123")
	if err != nil {
		t.Fatalf("PauseCronJob: %v", err)
	}
	if !paused.IsPaused() {
		t.Fatalf("paused job = %#v, want paused", paused)
	}
	if _, err := client.ResumeCronJob(context.Background(), "assistant", "job123"); err != nil {
		t.Fatalf("ResumeCronJob: %v", err)
	}
	if err := client.DeleteCronJob(context.Background(), "assistant", "job123"); err != nil {
		t.Fatalf("DeleteCronJob: %v", err)
	}
}

func writeCronJob(response http.ResponseWriter, paused bool) {
	response.Header().Set("Content-Type", "application/json")
	state := "scheduled"
	enabled := true
	if paused {
		state = "paused"
		enabled = false
	}
	job := map[string]any{
		"id": "job123", "profile": "assistant", "name": "Task check", "prompt": "Check my tasks",
		"schedule": map[string]string{"display": "every 1h"}, "schedule_display": "every 1h",
		"deliver": "local", "skills": []string{"morning-brief"}, "provider": "custom",
		"enabled": enabled, "state": state, "next_run_at": "2026-09-12T10:00:00Z",
	}
	_ = json.NewEncoder(response).Encode(job)
}
