package hermes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSetModelAcceptsSuccessfulApplicationResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/model/set":
			var assignment ModelAssignment
			if err := json.NewDecoder(request.Body).Decode(&assignment); err != nil {
				t.Fatalf("decode model assignment: %v", err)
			}
			if !assignment.ConfirmExpensiveModel {
				t.Fatalf("confirm_expensive_model = false, want true")
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":true,"scope":"main","provider":"custom","model":"local"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetModel(context.Background(), ModelAssignment{
		Scope:                 "main",
		Provider:              "custom",
		Model:                 "local",
		ConfirmExpensiveModel: true,
	}); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
}

func TestClientSetModelReportsConfirmationRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/auth/password-login":
			response.WriteHeader(http.StatusOK)
		case "/api/model/set":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"ok":false,"confirm_required":true,"confirm_message":"confirm model pricing"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "iac", "secret")
	if err != nil {
		t.Fatal(err)
	}
	err = client.SetModel(context.Background(), ModelAssignment{
		Scope:    "main",
		Provider: "custom",
		Model:    "expensive-local",
	})
	if err == nil {
		t.Fatal("SetModel returned nil, want application-level failure")
	}

	var operationError *OperationError
	if !errors.As(err, &operationError) {
		t.Fatalf("SetModel error = %T (%v), want OperationError", err, err)
	}
	if !operationError.ConfirmRequired || operationError.Message != "confirm model pricing" {
		t.Fatalf("operation error = %#v, want confirmation details", operationError)
	}
	if got := err.Error(); got != "hermes model assignment requires confirmation: confirm model pricing" {
		t.Fatalf("SetModel error text = %q", got)
	}
}
