package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientManagesCustomEndpointWithoutSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/password-login" && request.URL.Query().Get("profile") != "default" {
			t.Errorf("profile query = %q, want default", request.URL.Query().Get("profile"))
		}
		switch {
		case request.URL.Path == "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodGet && request.URL.Path == "/api/providers/custom-endpoints":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"endpoints":[{"id":"llama","name":"llama.cpp","base_url":"http://llama:8080/v1","model":"my-model","models":["my-model"],"context_length":32768,"discover_models":true,"has_api_key":false,"is_current":true,"source":"providers"}],"current":{"provider":"llama","model":"my-model","base_url":"http://llama:8080/v1"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/providers/custom-endpoints":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode custom endpoint request: %v", err)
			}
			if body["id"] != "llama" || body["name"] != "llama.cpp" || body["base_url"] != "http://llama:8080/v1" || body["model"] != "my-model" || body["discover_models"] != true {
				t.Errorf("custom endpoint request = %#v", body)
			}
			if _, ok := body["api_key"]; ok {
				t.Fatal("custom endpoint request unexpectedly contained api_key")
			}
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodDelete && request.URL.Path == "/api/providers/custom-endpoints/llama":
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
	result, err := client.ListCustomEndpoints(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListCustomEndpoints: %v", err)
	}
	if len(result.Endpoints) != 1 || result.Endpoints[0].ID != "llama" || result.Endpoints[0].ContextLength == nil || *result.Endpoints[0].ContextLength != 32768 || result.Endpoints[0].HasAPIKey {
		t.Fatalf("custom endpoints = %#v, want secret-free llama endpoint", result.Endpoints)
	}
	if err := client.UpsertCustomEndpoint(context.Background(), "default", CustomEndpointRequest{
		ID: "llama", Name: "llama.cpp", BaseURL: "http://llama:8080/v1", Model: "my-model", DiscoverModels: true,
	}); err != nil {
		t.Fatalf("UpsertCustomEndpoint: %v", err)
	}
	if err := client.DeleteCustomEndpoint(context.Background(), "default", "llama"); err != nil {
		t.Fatalf("DeleteCustomEndpoint: %v", err)
	}
}
