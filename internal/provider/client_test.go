package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func TestClientLogsInAndSetsModel(t *testing.T) {
	var loggedIn bool
	var received hermes.ModelAssignment
	var receivedProfile string

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "terraform-provider-hermes/dev" {
			t.Errorf("unexpected user agent: %q", request.Header.Get("User-Agent"))
		}
		switch request.URL.Path {
		case "/auth/password-login":
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["provider"] != "basic" || body["username"] != "iac" || body["password"] != "secret" {
				t.Fatalf("unexpected login body: %#v", body)
			}
			loggedIn = true
			http.SetCookie(response, &http.Cookie{Name: "hermes_session_at", Value: "test", Path: "/"})
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true}`))
		case "/api/model/set":
			if !loggedIn {
				t.Fatal("model endpoint was called before login")
			}
			receivedProfile = request.URL.Query().Get("profile")
			if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
				t.Error(err)
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	err = client.SetModel(context.Background(), hermes.ModelAssignment{
		Scope:    "main",
		Provider: "custom",
		Model:    "my-model",
		BaseURL:  "http://llm.example.com:8000/v1",
		Profile:  "terraform-acceptance",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.Provider != "custom" || received.Model != "my-model" {
		t.Fatalf("unexpected assignment: %#v", received)
	}
	if !strings.Contains(received.BaseURL, "llm.example.com") {
		t.Fatalf("unexpected base URL: %q", received.BaseURL)
	}
	if receivedProfile != "terraform-acceptance" {
		t.Fatalf("profile query = %q, want terraform-acceptance", receivedProfile)
	}
}

func TestClientGetsTypedConfigAndProfile(t *testing.T) {
	var gotProfile string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			http.SetCookie(response, &http.Cookie{Name: "hermes_session_at", Value: "test", Path: "/"})
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true}`))
		case "/api/config":
			gotProfile = request.URL.Query().Get("profile")
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{
				"model": {"provider": "custom", "default": "local-model", "base_url": "http://llama/v1"},
				"auxiliary": {"vision": {"provider": "custom", "model": "vision-model"}}
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
	config, err := client.GetConfig(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if gotProfile != "default" {
		t.Fatalf("profile query = %q, want default", gotProfile)
	}
	assignment, found := config.FindModelAssignment("main", "")
	if !found || assignment.Provider != "custom" || assignment.Model != "local-model" {
		t.Fatalf("unexpected main assignment: %#v (found=%t)", assignment, found)
	}
}

func TestClientRetriesExpiredSession(t *testing.T) {
	loginCount := 0
	configCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			loginCount++
			http.SetCookie(response, &http.Cookie{Name: "hermes_session_at", Value: fmt.Sprintf("session-%d", loginCount), Path: "/"})
			response.WriteHeader(http.StatusOK)
		case "/api/config":
			configCount++
			if configCount == 1 {
				response.WriteHeader(http.StatusUnauthorized)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"model":"local-model"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetConfig(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if loginCount != 2 || configCount != 2 {
		t.Fatalf("login count = %d, config count = %d; want two of each", loginCount, configCount)
	}
}

func TestClientDoesNotExposeErrorResponseBody(t *testing.T) {
	const secret = "do-not-log-this-secret"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/config":
			response.WriteHeader(http.StatusInternalServerError)
			_, _ = response.Write([]byte(secret))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetConfig(context.Background(), "")
	if err == nil {
		t.Fatal("expected GetConfig to fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed the response body: %v", err)
	}
}
