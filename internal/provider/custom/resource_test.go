package custom

import (
	"context"
	"reflect"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func TestSchemaContract(t *testing.T) {
	var response frameworkresource.SchemaResponse
	NewResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", response.Diagnostics)
	}
	if diagnostics := response.Schema.ValidateImplementation(context.Background()); diagnostics.HasError() {
		t.Fatalf("schema validation diagnostics: %v", diagnostics)
	}
	for _, name := range []string{"id", "profile", "name", "base_url", "model", "discover_models", "context_length", "models", "has_api_key", "is_current", "source"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("schema is missing %q", name)
		}
	}
}

func TestEndpointIDAndValidation(t *testing.T) {
	if got := endpointID("Llama.cpp / local"); got != "llama-cpp-local" {
		t.Fatalf("endpointID = %q, want llama-cpp-local", got)
	}
	valid := resourceModel{
		Profile: types.StringValue("default"), Name: types.StringValue("llama.cpp"),
		BaseURL: types.StringValue("http://llm.example.com:8000/v1"), Model: types.StringValue("local-model"),
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid custom provider: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*resourceModel)
	}{
		{name: "invalid profile", mutate: func(model *resourceModel) { model.Profile = types.StringValue("Assistant") }},
		{name: "invalid URL", mutate: func(model *resourceModel) { model.BaseURL = types.StringValue("llama:8080") }},
		{name: "missing model", mutate: func(model *resourceModel) { model.Model = types.StringValue("") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateConfig(candidate); err == nil {
				t.Fatalf("validateConfig(%#v) succeeded, want an error", candidate)
			}
		})
	}
}

func TestResourceCreateUpdateAndReadLifecycle(t *testing.T) {
	client := &fakeCustomProviderClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile: types.StringValue("default"), Name: types.StringValue("llama.cpp"),
		BaseURL: types.StringValue("http://llm.example.com:8000/v1"), Model: types.StringValue("my-model"),
		DiscoverModels: types.BoolValue(true), ContextLength: types.Int64Null(), Models: types.ListNull(types.StringType),
	}
	planState := customState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: planState}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(planState)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	if client.request.ID != "llama-cpp" || client.request.Name != "llama.cpp" || client.request.BaseURL != "http://llm.example.com:8000/v1" {
		t.Fatalf("upsert request = %#v", client.request)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("decode Create state: %v", createResponse.Diagnostics)
	}
	if created.ID.ValueString() != "default:llama-cpp" {
		t.Fatalf("created state = %#v, want canonical provider identity", created)
	}

	updatedPlan := created
	updatedPlan.Name = types.StringValue("local llama")
	updatedPlan.Model = types.StringValue("my-other-model")
	updatedPlanState := customState(t, updatedPlan)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{
		Plan: tfsdk.Plan(updatedPlanState), State: createResponse.State,
	}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.request.ID != "llama-cpp" || client.request.Name != "local llama" || client.request.Model != "my-other-model" {
		t.Fatalf("updated request = %#v, want stable ID and changed metadata", client.request)
	}
}

func TestBuildEndpointRequestDoesNotIncludeSecret(t *testing.T) {
	model := resourceModel{
		Name: types.StringValue("llama"), BaseURL: types.StringValue("http://llama/v1"),
		Model: types.StringValue("local"), DiscoverModels: types.BoolValue(true),
	}
	request := endpointRequest("llama", model)
	if request.ID != "llama" || request.Name != "llama" || request.BaseURL != "http://llama/v1" || request.Model != "local" || !request.DiscoverModels {
		t.Fatalf("endpoint request = %#v", request)
	}
	if reflect.TypeOf(request).NumField() != 7 {
		t.Fatalf("endpoint request fields = %d, want the documented non-secret contract", reflect.TypeOf(request).NumField())
	}
}

type fakeCustomProviderClient struct {
	request hermes.CustomEndpointRequest
	result  hermes.CustomEndpointsResponse
}

func (f *fakeCustomProviderClient) ListCustomEndpoints(context.Context, string) (hermes.CustomEndpointsResponse, error) {
	if len(f.result.Endpoints) == 0 {
		f.result.Endpoints = []hermes.CustomEndpoint{{
			ID: "llama-cpp", Name: f.request.Name, BaseURL: f.request.BaseURL, Model: f.request.Model,
			DiscoverModels: true, Models: []string{"my-model"}, Source: "providers",
		}}
	}
	return f.result, nil
}

func (f *fakeCustomProviderClient) UpsertCustomEndpoint(_ context.Context, _ string, request hermes.CustomEndpointRequest) error {
	f.request = request
	f.result = hermes.CustomEndpointsResponse{}
	return nil
}

func (f *fakeCustomProviderClient) DeleteCustomEndpoint(context.Context, string, string) error {
	f.result = hermes.CustomEndpointsResponse{}
	return nil
}

func customState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), customTerraformValue(t, model)),
		Schema: schema,
	}
}

func customTerraformValue(t *testing.T, model resourceModel) map[string]tftypes.Value {
	t.Helper()
	return map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, terraformString(model.ID)),
		"profile":         tftypes.NewValue(tftypes.String, terraformString(model.Profile)),
		"name":            tftypes.NewValue(tftypes.String, terraformString(model.Name)),
		"base_url":        tftypes.NewValue(tftypes.String, terraformString(model.BaseURL)),
		"model":           tftypes.NewValue(tftypes.String, terraformString(model.Model)),
		"discover_models": tftypes.NewValue(tftypes.Bool, terraformBool(model.DiscoverModels)),
		"context_length":  tftypes.NewValue(tftypes.Number, terraformInt64(model.ContextLength)),
		"models":          terraformList(t, model.Models),
		"has_api_key":     tftypes.NewValue(tftypes.Bool, terraformBool(model.HasAPIKey)),
		"is_current":      tftypes.NewValue(tftypes.Bool, terraformBool(model.IsCurrent)),
		"source":          tftypes.NewValue(tftypes.String, terraformString(model.Source)),
	}
}

func terraformList(t *testing.T, value types.List) tftypes.Value {
	t.Helper()
	terraformValue, err := value.ToTerraformValue(context.Background())
	if err != nil {
		t.Fatalf("convert list to Terraform value: %v", err)
	}
	return terraformValue
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

func terraformInt64(value types.Int64) any {
	if value.IsNull() {
		return nil
	}
	return value.ValueInt64()
}
