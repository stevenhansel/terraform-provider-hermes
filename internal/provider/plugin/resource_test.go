package plugin

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
	for _, name := range []string{"id", "identifier", "catalog_name", "ref", "force", "enabled", "name", "version", "description", "source", "runtime_status", "has_dashboard_manifest", "can_remove", "can_update_git", "auth_required", "auth_command", "user_hidden", "removed_reason"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("plugin schema is missing %q", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	valid := resourceModel{Identifier: types.StringValue("example/calendar"), Ref: types.StringValue("0123456789012345678901234567890123456789"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid plugin config: %v", err)
	}
	for _, test := range []struct {
		name  string
		model resourceModel
	}{
		{name: "neither source", model: resourceModel{Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "both sources", model: resourceModel{Identifier: types.StringValue("example/calendar"), CatalogName: types.StringValue("calendar"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "local source", model: resourceModel{Identifier: types.StringValue("file:///tmp/plugin"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "insecure source", model: resourceModel{Identifier: types.StringValue("http://example/plugin.git"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "short ref", model: resourceModel{Identifier: types.StringValue("example/calendar"), Ref: types.StringValue("main"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "unpinned custom source", model: resourceModel{Identifier: types.StringValue("example/calendar"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
		{name: "catalog ref", model: resourceModel{CatalogName: types.StringValue("calendar"), Ref: types.StringValue("0123456789012345678901234567890123456789"), Enabled: types.BoolValue(true), Force: types.BoolValue(false)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateConfig(test.model); err == nil {
				t.Fatalf("validateConfig(%#v) succeeded, want error", test.model)
			}
		})
	}
}

func TestPluginIDRoundTrip(t *testing.T) {
	name := "category/calendar"
	id := pluginID(name)
	if id != "plugin:category%2Fcalendar" {
		t.Fatalf("pluginID = %q", id)
	}
	parsed, err := parseID(id)
	if err != nil || parsed != name {
		t.Fatalf("parseID = (%q, %v), want %q", parsed, err, name)
	}
}

func TestResourceCreateUpdateAndDelete(t *testing.T) {
	client := &fakePluginClient{plugins: []hermes.AgentPlugin{{
		Name: "calendar", Version: "1.2.3", Description: "Calendar tools", Source: "user",
		RuntimeStatus: "enabled", CanRemove: true, CanUpdateGit: true,
	}}}
	instance := &Resource{client: client}
	plan := resourceModel{
		Identifier: types.StringValue("example/calendar"),
		Ref:        types.StringValue("0123456789012345678901234567890123456789"),
		Force:      types.BoolValue(false),
		Enabled:    types.BoolValue(true),
		Name:       types.StringNull(),
	}
	state := pluginState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: state}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() || created.Name.ValueString() != "calendar" || created.ID.ValueString() != "plugin:calendar" {
		t.Fatalf("created state = %#v, diagnostics = %v", created, createResponse.Diagnostics)
	}

	updated := created
	updated.Enabled = types.BoolValue(false)
	updateState := pluginState(t, updated)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{Plan: tfsdk.Plan(updateState), State: createResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() || client.lastEnabled == nil || *client.lastEnabled {
		t.Fatalf("Update diagnostics = %v, lastEnabled = %#v", updateResponse.Diagnostics, client.lastEnabled)
	}

	deleteResponse := frameworkresource.DeleteResponse{State: updateResponse.State}
	instance.Delete(context.Background(), frameworkresource.DeleteRequest{State: updateResponse.State}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() || client.deleteCalls != 1 {
		t.Fatalf("Delete diagnostics = %v, deleteCalls = %d", deleteResponse.Diagnostics, client.deleteCalls)
	}
}

type fakePluginClient struct {
	plugins      []hermes.AgentPlugin
	installCalls int
	deleteCalls  int
	lastEnabled  *bool
}

func (f *fakePluginClient) ListAgentPlugins(context.Context) ([]hermes.AgentPlugin, error) {
	return append([]hermes.AgentPlugin(nil), f.plugins...), nil
}

func (f *fakePluginClient) InstallAgentPlugin(_ context.Context, request hermes.AgentPluginInstallRequest) (hermes.AgentPluginMutationResult, error) {
	f.installCalls++
	return hermes.AgentPluginMutationResult{OK: true, PluginName: "calendar", Enabled: request.Enable}, nil
}

func (f *fakePluginClient) SetAgentPluginEnabled(_ context.Context, name string, enabled bool) (hermes.AgentPluginMutationResult, error) {
	f.lastEnabled = &enabled
	for index := range f.plugins {
		if f.plugins[index].Name == name {
			f.plugins[index].RuntimeStatus = "disabled"
			if enabled {
				f.plugins[index].RuntimeStatus = "enabled"
			}
		}
	}
	return hermes.AgentPluginMutationResult{OK: true, Name: name, Enabled: enabled}, nil
}

func (f *fakePluginClient) DeleteAgentPlugin(_ context.Context, name string) (hermes.AgentPluginMutationResult, error) {
	f.deleteCalls++
	for index := range f.plugins {
		if f.plugins[index].Name == name {
			f.plugins = append(f.plugins[:index], f.plugins[index+1:]...)
		}
	}
	return hermes.AgentPluginMutationResult{OK: true, Name: name}, nil
}

func pluginState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), pluginTerraformValue(model)),
		Schema: schema,
	}
}

func pluginTerraformValue(model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":                     terraformString(model.ID),
		"identifier":             terraformString(model.Identifier),
		"catalog_name":           terraformString(model.CatalogName),
		"ref":                    terraformString(model.Ref),
		"force":                  terraformBool(model.Force),
		"enabled":                terraformBool(model.Enabled),
		"name":                   terraformString(model.Name),
		"version":                terraformString(model.Version),
		"description":            terraformString(model.Description),
		"source":                 terraformString(model.Source),
		"runtime_status":         terraformString(model.RuntimeStatus),
		"has_dashboard_manifest": terraformBool(model.HasDashboardManifest),
		"can_remove":             terraformBool(model.CanRemove),
		"can_update_git":         terraformBool(model.CanUpdateGit),
		"auth_required":          terraformBool(model.AuthRequired),
		"auth_command":           terraformString(model.AuthCommand),
		"user_hidden":            terraformBool(model.UserHidden),
		"removed_reason":         terraformString(model.RemovedReason),
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
