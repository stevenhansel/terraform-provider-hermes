package model

import (
	"context"
	"encoding/json"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestFindModelAssignmentFromCustomProvider(t *testing.T) {
	config := decodeDashboardConfig(t, `{
		"model": "my-model",
		"custom_providers": [{
			"name": "Llm.example.com:8000",
			"base_url": "http://llm.example.com:8000/v1",
			"model": "my-model"
		}]
	}`)

	assignment, found := config.FindModelAssignment("main", "")
	if !found {
		t.Fatal("expected the main model assignment to be found")
	}
	if assignment.Provider != "custom" {
		t.Fatalf("provider = %q, want custom", assignment.Provider)
	}
	if assignment.Model != "my-model" {
		t.Fatalf("model = %q, want my-model", assignment.Model)
	}
	if assignment.BaseURL != "http://llm.example.com:8000/v1" {
		t.Fatalf("base URL = %q, want the configured llama endpoint", assignment.BaseURL)
	}
	if !assignment.BaseURLKnown {
		t.Fatal("expected the custom provider base URL to be known")
	}
}

func TestFindModelAssignmentFromObjectConfig(t *testing.T) {
	config := decodeDashboardConfig(t, `{
		"model": {
			"provider": "openai",
			"default": "gpt-4.1",
			"base_url": "https://api.openai.com/v1"
		}
	}`)

	assignment, found := config.FindModelAssignment("main", "")
	if !found {
		t.Fatal("expected the object-shaped main model assignment to be found")
	}
	want := modelAssignmentState{
		Scope:        "main",
		Provider:     "openai",
		Model:        "gpt-4.1",
		BaseURL:      "https://api.openai.com/v1",
		BaseURLKnown: true,
	}
	if assignment != want {
		t.Fatalf("assignment = %#v, want %#v", assignment, want)
	}
}

func TestFindModelAssignmentFromAuxiliaryConfig(t *testing.T) {
	config := decodeDashboardConfig(t, `{
		"auxiliary": {
			"transient_retries": 2,
			"free_only": false,
			"vision": {
				"provider": "custom",
				"model": "vision-model"
			}
		}
	}`)

	assignment, found := config.FindModelAssignment("auxiliary", "vision")
	if !found {
		t.Fatal("expected the auxiliary model assignment to be found")
	}
	if assignment.Scope != "auxiliary" || assignment.Task != "vision" || assignment.Provider != "custom" || assignment.Model != "vision-model" {
		t.Fatalf("unexpected auxiliary assignment: %#v", assignment)
	}
	if assignment.BaseURLKnown {
		t.Fatal("expected the absent auxiliary base URL to remain unknown")
	}
}

func TestFindModelAssignmentPreservesUnknownBaseURL(t *testing.T) {
	config := decodeDashboardConfig(t, `{
		"model": "local-model"
	}`)

	assignment, found := config.FindModelAssignment("main", "")
	if !found {
		t.Fatal("expected a non-empty raw model assignment to be found")
	}
	if assignment.Provider != "custom" || assignment.Model != "local-model" || assignment.BaseURLKnown {
		t.Fatalf("unexpected raw assignment: %#v", assignment)
	}
}

func TestDashboardConfigRejectsInvalidModelShape(t *testing.T) {
	var config dashboardConfig
	if err := json.Unmarshal([]byte(`{"model": 42}`), &config); err == nil {
		t.Fatal("expected an invalid model shape to fail decoding")
	}
}

func TestParseModelID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantScope   string
		wantTask    string
		wantProfile string
		wantError   bool
	}{
		{name: "main", id: "main", wantScope: "main"},
		{name: "auxiliary", id: "auxiliary:vision", wantScope: "auxiliary", wantTask: "vision"},
		{name: "profile main", id: "default:main", wantScope: "main", wantProfile: "default"},
		{name: "profile auxiliary", id: "default:auxiliary:vision", wantScope: "auxiliary", wantTask: "vision", wantProfile: "default"},
		{name: "empty", id: "", wantError: true},
		{name: "unknown main assignment", id: "primary", wantError: true},
		{name: "missing auxiliary task", id: "auxiliary:", wantError: true},
		{name: "invalid profile assignment", id: "default:primary", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope, task, profile, err := parseModelID(test.id)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected an error for %q", test.id)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseModelID(%q): %v", test.id, err)
			}
			if scope != test.wantScope || task != test.wantTask || profile != test.wantProfile {
				t.Fatalf("parseModelID(%q) = (%q, %q, %q), want (%q, %q, %q)", test.id, scope, task, profile, test.wantScope, test.wantTask, test.wantProfile)
			}
		})
	}
}

func TestValidateModel(t *testing.T) {
	tests := []struct {
		name      string
		model     modelAssignmentModel
		wantError bool
	}{
		{
			name: "defaults to main",
			model: modelAssignmentModel{
				ModelProvider: types.StringValue("custom"),
				Model:         types.StringValue("local-model"),
			},
		},
		{
			name: "valid auxiliary",
			model: modelAssignmentModel{
				Scope:         types.StringValue("AUXILIARY"),
				Task:          types.StringValue("vision"),
				ModelProvider: types.StringValue("custom"),
				Model:         types.StringValue("vision-model"),
			},
		},
		{
			name: "auxiliary requires task",
			model: modelAssignmentModel{
				Scope:         types.StringValue("auxiliary"),
				ModelProvider: types.StringValue("custom"),
				Model:         types.StringValue("vision-model"),
			},
			wantError: true,
		},
		{
			name: "main rejects task",
			model: modelAssignmentModel{
				Scope:         types.StringValue("main"),
				Task:          types.StringValue("vision"),
				ModelProvider: types.StringValue("custom"),
				Model:         types.StringValue("local-model"),
			},
			wantError: true,
		},
		{
			name: "unknown values are deferred",
			model: modelAssignmentModel{
				Scope:         types.StringUnknown(),
				ModelProvider: types.StringUnknown(),
				Model:         types.StringUnknown(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateModel(&test.model)
			if (err != nil) != test.wantError {
				t.Fatalf("validateModel() error = %v, wantError = %t", err, test.wantError)
			}
		})
	}
}

func TestModelAssignmentReadPreservesUnknownBaseURL(t *testing.T) {
	resource := &modelAssignmentResource{
		client: fakeModelClient{
			config: decodeDashboardConfig(t, `{"model":"local-model"}`),
		},
	}
	initial := modelAssignmentModel{
		ID:            types.StringValue("main"),
		Scope:         types.StringValue("main"),
		ModelProvider: types.StringValue("custom"),
		Model:         types.StringValue("previous-model"),
		BaseURL:       types.StringValue("http://llama/v1"),
	}
	request := frameworkReadRequest(t, initial)
	response := frameworkReadResponse(request)

	resource.Read(context.Background(), request, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", response.Diagnostics)
	}

	var got modelAssignmentModel
	response.Diagnostics.Append(response.State.Get(context.Background(), &got)...)
	if response.Diagnostics.HasError() {
		t.Fatalf("decode Read state: %v", response.Diagnostics)
	}
	if got.Model.ValueString() != "local-model" {
		t.Fatalf("model = %q, want local-model", got.Model.ValueString())
	}
	if got.BaseURL.ValueString() != "http://llama/v1" {
		t.Fatalf("base_url = %q, want the previous configured value", got.BaseURL.ValueString())
	}
}

func decodeDashboardConfig(t *testing.T, value string) dashboardConfig {
	t.Helper()
	var config dashboardConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		t.Fatalf("decode dashboard config: %v", err)
	}
	return config
}

type fakeModelClient struct {
	config dashboardConfig
}

func (f fakeModelClient) GetConfig(_ context.Context, _ string) (dashboardConfig, error) {
	return f.config, nil
}

func (fakeModelClient) SetModel(_ context.Context, _ modelAssignment) error {
	return nil
}

func frameworkReadRequest(t *testing.T, model modelAssignmentModel) frameworkresource.ReadRequest {
	t.Helper()
	schema := modelAssignmentSchema(context.Background(), frameworkresource.SchemaRequest{})
	return frameworkresource.ReadRequest{
		State: tfsdk.State{
			Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), modelTerraformValue(model)),
			Schema: schema,
		},
	}
}

func frameworkReadResponse(request frameworkresource.ReadRequest) frameworkresource.ReadResponse {
	return frameworkresource.ReadResponse{State: request.State}
}

func modelTerraformValue(model modelAssignmentModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":                      tftypes.NewValue(tftypes.String, terraformString(model.ID)),
		"scope":                   tftypes.NewValue(tftypes.String, terraformString(model.Scope)),
		"task":                    tftypes.NewValue(tftypes.String, terraformString(model.Task)),
		"model_provider":          tftypes.NewValue(tftypes.String, terraformString(model.ModelProvider)),
		"model":                   tftypes.NewValue(tftypes.String, terraformString(model.Model)),
		"base_url":                tftypes.NewValue(tftypes.String, terraformString(model.BaseURL)),
		"api_key":                 tftypes.NewValue(tftypes.String, terraformString(model.APIKey)),
		"confirm_expensive_model": tftypes.NewValue(tftypes.Bool, terraformBool(model.ConfirmExpensiveModel)),
		"profile":                 tftypes.NewValue(tftypes.String, terraformString(model.Profile)),
	}
}

func terraformString(value types.String) any {
	if value.IsNull() {
		return nil
	}
	return value.ValueString()
}

func terraformBool(value types.Bool) any {
	if value.IsNull() {
		return nil
	}
	return value.ValueBool()
}
