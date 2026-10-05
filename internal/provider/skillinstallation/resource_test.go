package skillinstallation

import (
	"context"
	"testing"
	"time"

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
	for _, name := range []string{"id", "profile", "identifier", "name", "installed", "trust_level", "scan_verdict", "last_action"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("skill installation schema is missing %q", name)
		}
	}
}

func TestValidateConfigAndID(t *testing.T) {
	valid := resourceModel{
		Profile:    types.StringValue("assistant"),
		Identifier: types.StringValue("official/productivity/morning-brief"),
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid skill installation config: %v", err)
	}
	id := installationID("assistant", "official/productivity/morning-brief")
	if id != "assistant:official%2Fproductivity%2Fmorning-brief" {
		t.Fatalf("installationID = %q", id)
	}
	profile, identifier, err := parseID(id)
	if err != nil || profile != "assistant" || identifier != "official/productivity/morning-brief" {
		t.Fatalf("parseID = (%q, %q, %v)", profile, identifier, err)
	}
	for _, invalid := range []string{"", "assistant", "Assistant:official/skill", "assistant:../skill", "assistant:skill name"} {
		if _, _, err := parseID(invalid); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", invalid)
		}
	}
}

func TestResourceAdoptsInstallsAndUninstalls(t *testing.T) {
	client := &fakeSkillInstallationClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile:    types.StringValue("assistant"),
		Identifier: types.StringValue("official/productivity/morning-brief"),
		LastAction: types.StringNull(),
	}
	state := installationState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: state}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	if client.installCalls != 1 || client.waitCalls != 1 {
		t.Fatalf("install calls = %d, wait calls = %d, want one each", client.installCalls, client.waitCalls)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() || created.Name.ValueString() != "morning-brief" || !created.Installed.ValueBool() || created.ScanVerdict.ValueString() != "safe" {
		t.Fatalf("created state = %#v, diagnostics = %v", created, createResponse.Diagnostics)
	}

	deleteResponse := frameworkresource.DeleteResponse{State: createResponse.State}
	instance.Delete(context.Background(), frameworkresource.DeleteRequest{State: createResponse.State}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() {
		t.Fatalf("Delete diagnostics: %v", deleteResponse.Diagnostics)
	}
	if client.uninstallCalls != 1 || client.waitCalls != 2 {
		t.Fatalf("uninstall calls = %d, wait calls = %d, want one uninstall and two waits", client.uninstallCalls, client.waitCalls)
	}
}

type fakeSkillInstallationClient struct {
	installed      bool
	installCalls   int
	uninstallCalls int
	waitCalls      int
}

func (f *fakeSkillInstallationClient) ListSkillHubSources(context.Context, string) (hermes.SkillHubSources, error) {
	if !f.installed {
		return hermes.SkillHubSources{Installed: map[string]hermes.SkillHubInstallation{}}, nil
	}
	return hermes.SkillHubSources{Installed: map[string]hermes.SkillHubInstallation{
		"official/productivity/morning-brief": {
			Identifier: "official/productivity/morning-brief", Name: "morning-brief", TrustLevel: "official", ScanVerdict: "safe",
		},
	}}, nil
}

func (f *fakeSkillInstallationClient) StartSkillInstall(context.Context, string, string) (string, error) {
	f.installCalls++
	f.installed = true
	return "skills-install-morning-brief", nil
}

func (f *fakeSkillInstallationClient) StartSkillUninstall(context.Context, string, string) (string, error) {
	f.uninstallCalls++
	f.installed = false
	return "skills-uninstall-morning-brief", nil
}

func (f *fakeSkillInstallationClient) WaitForAction(_ context.Context, _ string, timeout time.Duration) error {
	f.waitCalls++
	if timeout != installationActionTimeout {
		return testingError("unexpected action timeout")
	}
	return nil
}

type testingError string

func (e testingError) Error() string { return string(e) }

func installationState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), installationTerraformValue(model)),
		Schema: schema,
	}
}

func installationTerraformValue(model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":           terraformString(model.ID),
		"profile":      terraformString(model.Profile),
		"identifier":   terraformString(model.Identifier),
		"name":         terraformString(model.Name),
		"installed":    terraformBool(model.Installed),
		"trust_level":  terraformString(model.TrustLevel),
		"scan_verdict": terraformString(model.ScanVerdict),
		"last_action":  terraformString(model.LastAction),
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
