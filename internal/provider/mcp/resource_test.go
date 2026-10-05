package mcp

import (
	"context"
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
	for _, name := range []string{"id", "profile", "name", "url", "auth", "bearer_token_env", "enabled", "transport"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("schema is missing %q", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	valid := resourceModel{
		Profile: types.StringValue("assistant"),
		Name:    types.StringValue("docs"),
		URL:     types.StringValue("https://docs.example.com/mcp"),
		Auth:    types.StringValue("oauth"),
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid MCP config: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*resourceModel)
	}{
		{name: "default profile is still explicit but valid", mutate: func(model *resourceModel) { model.Profile = types.StringValue("default") }},
		{name: "invalid profile", mutate: func(model *resourceModel) { model.Profile = types.StringValue("Assistant") }},
		{name: "invalid name", mutate: func(model *resourceModel) { model.Name = types.StringValue("docs/server") }},
		{name: "invalid URL", mutate: func(model *resourceModel) { model.URL = types.StringValue("docs") }},
		{name: "header requires an environment reference", mutate: func(model *resourceModel) { model.Auth = types.StringValue("header") }},
		{name: "bearer reference requires header auth", mutate: func(model *resourceModel) {
			model.BearerTokenEnv = types.StringValue("DOCS_MCP_TOKEN")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			err := validateConfig(candidate)
			if test.name == "default profile is still explicit but valid" {
				if err != nil {
					t.Fatalf("default profile: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateConfig(%#v) succeeded, want an error", candidate)
			}
		})
	}
}

func TestBearerTokenFromPlanReadsOnlyNamedEnvironmentVariable(t *testing.T) {
	t.Setenv("HERMES_TEST_MCP_TOKEN", "token-value")
	token, err := bearerTokenFromPlan(resourceModel{
		Auth:           types.StringValue("header"),
		BearerTokenEnv: types.StringValue("HERMES_TEST_MCP_TOKEN"),
	})
	if err != nil || token != "token-value" {
		t.Fatalf("bearerTokenFromPlan = (%q, %v)", token, err)
	}

	if _, err := bearerTokenFromPlan(resourceModel{
		Auth:           types.StringValue("header"),
		BearerTokenEnv: types.StringValue("HERMES_MISSING_MCP_TOKEN"),
	}); err == nil {
		t.Fatal("missing bearer token environment variable unexpectedly succeeded")
	}

}

func TestResourceCreateUpdateAndReadLifecycle(t *testing.T) {
	client := &fakeMCPClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile: types.StringValue("assistant"),
		Name:    types.StringValue("docs"),
		URL:     types.StringValue("http://docs.example.com/mcp"),
		Auth:    types.StringValue("none"),
		Enabled: types.BoolValue(true),
	}
	planState := mcpState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: planState}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(planState)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("decode Create state: %v", createResponse.Diagnostics)
	}
	if created.ID.ValueString() != "assistant:docs" || created.Transport.ValueString() != "http" {
		t.Fatalf("created state = %#v, want profile-scoped HTTP identity", created)
	}

	updatedPlan := created
	updatedPlan.Enabled = types.BoolValue(false)
	updatedPlanState := mcpState(t, updatedPlan)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{
		Plan:  tfsdk.Plan(updatedPlanState),
		State: createResponse.State,
	}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.servers[0].Enabled {
		t.Fatal("MCP server remained enabled after update")
	}

	readRequest := frameworkresource.ReadRequest{State: updateResponse.State}
	readResponse := frameworkresource.ReadResponse{State: readRequest.State}
	instance.Read(context.Background(), readRequest, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", readResponse.Diagnostics)
	}
	var read resourceModel
	readResponse.Diagnostics.Append(readResponse.State.Get(context.Background(), &read)...)
	if readResponse.Diagnostics.HasError() || read.Enabled.ValueBool() {
		t.Fatalf("read state = %#v, want disabled server", read)
	}
}

func TestResourceCreateHeaderAuthenticationUsesEnvironmentReference(t *testing.T) {
	t.Setenv("HERMES_TEST_MCP_TOKEN", "secret-token")
	client := &fakeMCPClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile:        types.StringValue("assistant"),
		Name:           types.StringValue("homeassistant"),
		URL:            types.StringValue("http://homeassistant/mcp"),
		Auth:           types.StringValue("header"),
		BearerTokenEnv: types.StringValue("HERMES_TEST_MCP_TOKEN"),
		Enabled:        types.BoolValue(true),
	}
	response := frameworkresource.CreateResponse{State: mcpState(t, plan)}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(response.State)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", response.Diagnostics)
	}
	if len(client.requests) != 1 || client.requests[0].BearerToken != "secret-token" {
		t.Fatalf("create requests = %#v, want one request with resolved token", client.requests)
	}
	if client.requests[0].BearerToken == stringValue(plan.BearerTokenEnv) {
		t.Fatal("provider sent the environment variable name as the bearer token")
	}
	var state resourceModel
	response.Diagnostics.Append(response.State.Get(context.Background(), &state)...)
	if response.Diagnostics.HasError() {
		t.Fatalf("decode Create state: %v", response.Diagnostics)
	}
	if state.BearerTokenEnv.ValueString() != "HERMES_TEST_MCP_TOKEN" {
		t.Fatalf("bearer token state = %q, want environment reference", state.BearerTokenEnv.ValueString())
	}
}

func TestReadRemovesMissingServer(t *testing.T) {
	instance := &Resource{client: &fakeMCPClient{}}
	state := mcpState(t, resourceModel{
		ID: types.StringValue("assistant:docs"), Profile: types.StringValue("assistant"), Name: types.StringValue("docs"),
		URL: types.StringValue("http://docs/mcp"), Auth: types.StringValue("none"), Enabled: types.BoolValue(true),
	})
	response := frameworkresource.ReadResponse{State: state}
	instance.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", response.Diagnostics)
	}
	if !response.State.Raw.IsKnown() || !response.State.Raw.IsNull() {
		t.Fatal("missing server did not become a null state")
	}
}

func TestParseID(t *testing.T) {
	profile, name, err := parseID("assistant:docs")
	if err != nil || profile != "assistant" || name != "docs" {
		t.Fatalf("parseID = (%q, %q, %v)", profile, name, err)
	}
	for _, id := range []string{"", "assistant", "Assistant:docs", "assistant:docs/server"} {
		if _, _, err := parseID(id); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", id)
		}
	}
}

type fakeMCPClient struct {
	servers  []hermes.MCPServer
	requests []hermes.MCPServerRequest
}

func (f *fakeMCPClient) ListMCPServers(context.Context, string) ([]hermes.MCPServer, error) {
	return append([]hermes.MCPServer(nil), f.servers...), nil
}

func (f *fakeMCPClient) CreateMCPServer(_ context.Context, request hermes.MCPServerRequest) error {
	f.requests = append(f.requests, request)
	f.servers = append(f.servers, hermes.MCPServer{
		Name: request.Name, Transport: "http", URL: request.URL, Auth: request.Auth, Enabled: true,
	})
	return nil
}

func (f *fakeMCPClient) DeleteMCPServer(_ context.Context, _, name string) error {
	for index := range f.servers {
		if f.servers[index].Name == name {
			f.servers = append(f.servers[:index], f.servers[index+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeMCPClient) SetMCPServerEnabled(_ context.Context, _, name string, enabled bool) error {
	for index := range f.servers {
		if f.servers[index].Name == name {
			f.servers[index].Enabled = enabled
			return nil
		}
	}
	return nil
}

func mcpState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), mcpTerraformValue(model)),
		Schema: schema,
	}
}

func mcpTerraformValue(model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":               tftypes.NewValue(tftypes.String, terraformString(model.ID)),
		"profile":          tftypes.NewValue(tftypes.String, terraformString(model.Profile)),
		"name":             tftypes.NewValue(tftypes.String, terraformString(model.Name)),
		"url":              tftypes.NewValue(tftypes.String, terraformString(model.URL)),
		"auth":             tftypes.NewValue(tftypes.String, terraformString(model.Auth)),
		"bearer_token_env": tftypes.NewValue(tftypes.String, terraformString(model.BearerTokenEnv)),
		"enabled":          tftypes.NewValue(tftypes.Bool, terraformBool(model.Enabled)),
		"transport":        tftypes.NewValue(tftypes.String, terraformString(model.Transport)),
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
