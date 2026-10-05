package toolset

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
	for _, name := range []string{"id", "profile", "name", "enabled", "label", "description", "platform", "platform_label", "available", "configured", "tools", "post_setup_started"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("toolset schema is missing %q", name)
		}
	}
}

func TestValidateConfigAndID(t *testing.T) {
	valid := resourceModel{Profile: types.StringValue("assistant"), Name: types.StringValue("web"), Enabled: types.BoolValue(true)}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid toolset config: %v", err)
	}
	if profile, name, err := parseID("assistant:web"); err != nil || profile != "assistant" || name != "web" {
		t.Fatalf("parseID = (%q, %q, %v)", profile, name, err)
	}
	for _, invalid := range []string{"", "assistant", "Assistant:web", "assistant:Web", "assistant:tool set"} {
		if _, _, err := parseID(invalid); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", invalid)
		}
	}
}

func TestResourceCreateUpdateAndDelete(t *testing.T) {
	client := &fakeToolsetClient{toolsets: []hermes.Toolset{{
		Name: "web", Label: "Web", Description: "Browser tools", Platform: "cli", PlatformLabel: "CLI",
		Enabled: true, Available: true, Configured: true, Tools: []string{"browser", "search"},
	}}}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile:          types.StringValue("assistant"),
		Name:             types.StringValue("web"),
		Enabled:          types.BoolValue(true),
		Tools:            types.ListNull(types.StringType),
		PostSetupStarted: types.StringNull(),
	}
	state := toolsetState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: state}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() || created.ID.ValueString() != "assistant:web" || len(created.Tools.Elements()) != 2 {
		t.Fatalf("created state = %#v, diagnostics = %v", created, createResponse.Diagnostics)
	}

	updated := created
	updated.Enabled = types.BoolValue(false)
	updateState := toolsetState(t, updated)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{Plan: tfsdk.Plan(updateState), State: createResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.lastEnabled == nil || *client.lastEnabled {
		t.Fatalf("last enabled = %#v, want false", client.lastEnabled)
	}

	deleteResponse := frameworkresource.DeleteResponse{State: updateResponse.State}
	instance.Delete(context.Background(), frameworkresource.DeleteRequest{State: updateResponse.State}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() {
		t.Fatalf("Delete diagnostics: %v", deleteResponse.Diagnostics)
	}
	if client.lastEnabled == nil || *client.lastEnabled {
		t.Fatalf("delete enabled = %#v, want false", client.lastEnabled)
	}
}

type fakeToolsetClient struct {
	toolsets    []hermes.Toolset
	lastEnabled *bool
}

func (f *fakeToolsetClient) ListToolsets(context.Context, string) ([]hermes.Toolset, error) {
	return append([]hermes.Toolset(nil), f.toolsets...), nil
}

func (f *fakeToolsetClient) SetToolsetEnabled(_ context.Context, _ string, name string, enabled bool) (hermes.ToolsetToggleResult, error) {
	f.lastEnabled = &enabled
	for index := range f.toolsets {
		if f.toolsets[index].Name == name {
			f.toolsets[index].Enabled = enabled
		}
	}
	return hermes.ToolsetToggleResult{OK: true, Name: name, Enabled: enabled}, nil
}

func toolsetState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), toolsetTerraformValue(t, model)),
		Schema: schema,
	}
}

func toolsetTerraformValue(t *testing.T, model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":                 terraformString(model.ID),
		"profile":            terraformString(model.Profile),
		"name":               terraformString(model.Name),
		"enabled":            terraformBool(model.Enabled),
		"label":              terraformString(model.Label),
		"description":        terraformString(model.Description),
		"platform":           terraformString(model.Platform),
		"platform_label":     terraformString(model.PlatformLabel),
		"available":          terraformBool(model.Available),
		"configured":         terraformBool(model.Configured),
		"tools":              terraformList(t, model.Tools),
		"post_setup_started": terraformString(model.PostSetupStarted),
	}
}

func terraformString(value types.String) tftypes.Value {
	if value.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(tftypes.String, value.ValueString())
}

func terraformBool(value types.Bool) tftypes.Value {
	if value.IsNull() {
		return tftypes.NewValue(tftypes.Bool, nil)
	}
	return tftypes.NewValue(tftypes.Bool, value.ValueBool())
}

func terraformList(t *testing.T, value types.List) tftypes.Value {
	t.Helper()
	terraformValue, err := value.ToTerraformValue(context.Background())
	if err != nil {
		t.Fatalf("convert list to Terraform value: %v", err)
	}
	return terraformValue
}
