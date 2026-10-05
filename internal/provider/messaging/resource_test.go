package messaging

import (
	"context"
	"reflect"
	"strings"
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
	for _, name := range []string{"id", "profile", "platform", "enabled", "env_from", "configured", "gateway_running", "state", "error_code", "updated_at", "configured_keys"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("messaging schema is missing %q", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	envFrom, diagnostics := types.MapValueFrom(context.Background(), types.StringType, map[string]string{
		"HASS_URL":   "HERMES_HASS_URL",
		"HASS_TOKEN": "HERMES_HASS_TOKEN",
	})
	if diagnostics.HasError() {
		t.Fatalf("env_from value: %v", diagnostics)
	}
	valid := resourceModel{
		Profile: types.StringValue("assistant"), Platform: types.StringValue("homeassistant"),
		Enabled: types.BoolValue(true), EnvFrom: envFrom,
	}
	if err := validateConfig(context.Background(), valid); err != nil {
		t.Fatalf("valid messaging config: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*resourceModel)
	}{
		{name: "invalid profile", mutate: func(model *resourceModel) { model.Profile = types.StringValue("Assistant") }},
		{name: "invalid platform", mutate: func(model *resourceModel) { model.Platform = types.StringValue("Home Assistant") }},
		{name: "invalid Hermes environment key", mutate: func(model *resourceModel) {
			bad, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{"HASS-TOKEN": "HERMES_HASS_TOKEN"})
			model.EnvFrom = bad
		}},
		{name: "invalid provider environment name", mutate: func(model *resourceModel) {
			bad, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{"HASS_TOKEN": "not a variable"})
			model.EnvFrom = bad
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateConfig(context.Background(), candidate); err == nil {
				t.Fatalf("validateConfig(%#v) succeeded, want an error", candidate)
			}
		})
	}
}

func TestResourceCreateUpdateAndReadPreservesReferences(t *testing.T) {
	t.Setenv("HERMES_HASS_URL", "http://homeassistant.local:8123")
	t.Setenv("HERMES_HASS_TOKEN", "token-from-secret-manager")
	envFrom, diagnostics := types.MapValueFrom(context.Background(), types.StringType, map[string]string{
		"HASS_URL": "HERMES_HASS_URL", "HASS_TOKEN": "HERMES_HASS_TOKEN",
	})
	if diagnostics.HasError() {
		t.Fatalf("env_from value: %v", diagnostics)
	}
	client := &fakeMessagingClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Profile: types.StringValue("assistant"), Platform: types.StringValue("homeassistant"),
		Enabled: types.BoolValue(true), EnvFrom: envFrom, ConfiguredKeys: types.ListNull(types.StringType),
	}
	planState := messagingState(t, plan)
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	createResponse := frameworkresource.CreateResponse{State: tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), nil),
		Schema: schema,
	}}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(planState)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	if client.lastUpdate.Env["HASS_TOKEN"] != "token-from-secret-manager" {
		t.Fatalf("update env = %#v, want resolved token sent only to Hermes", client.lastUpdate.Env)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("decode Create state: %v", createResponse.Diagnostics)
	}
	if created.ID.ValueString() != "assistant:homeassistant" || created.EnvFrom.IsNull() || created.ConfiguredKeys.Elements()[0].(types.String).ValueString() != "HASS_TOKEN" {
		t.Fatalf("created state = %#v, want profile-scoped identity and configured keys", created)
	}
	stateValue := createResponse.State.Raw.String()
	if stateValue == "" || stateValue == "token-from-secret-manager" || strings.Contains(stateValue, "token-from-secret-manager") {
		t.Fatalf("state unexpectedly contains the raw token: %s", stateValue)
	}

	updatedPlan := created
	updatedPlan.Enabled = types.BoolValue(false)
	updatedState := messagingState(t, updatedPlan)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{Plan: tfsdk.Plan(updatedState), State: createResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.lastUpdate.Enabled == nil || *client.lastUpdate.Enabled {
		t.Fatalf("update = %#v, want disable-only update", client.lastUpdate)
	}
}

func TestBuildUpdateClearsRemovedOwnedKeys(t *testing.T) {
	left, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{
		"HASS_URL": "HERMES_HASS_URL", "HASS_TOKEN": "HERMES_HASS_TOKEN",
	})
	right, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{
		"HASS_TOKEN": "HERMES_HASS_TOKEN",
	})
	t.Setenv("HERMES_HASS_TOKEN", "token-from-secret-manager")
	update, err := buildUpdate(context.Background(), resourceModel{Enabled: types.BoolValue(true), EnvFrom: right}, resourceModel{Enabled: types.BoolValue(true), EnvFrom: left})
	if err != nil {
		t.Fatalf("buildUpdate: %v", err)
	}
	if update.Env["HASS_TOKEN"] != "token-from-secret-manager" || !reflect.DeepEqual(update.ClearEnv, []string{"HASS_URL"}) {
		t.Fatalf("update = %#v, want token refresh and removed-key clear", update)
	}
}

func TestMissingPlatformRemovesState(t *testing.T) {
	instance := &Resource{client: &fakeMessagingClient{}}
	state := messagingState(t, resourceModel{ID: types.StringValue("assistant:homeassistant"), Profile: types.StringValue("assistant"), Platform: types.StringValue("homeassistant"), Enabled: types.BoolValue(true), EnvFrom: types.MapNull(types.StringType), ConfiguredKeys: types.ListNull(types.StringType)})
	response := frameworkresource.ReadResponse{State: state}
	instance.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", response.Diagnostics)
	}
	if !response.State.Raw.IsNull() {
		t.Fatal("missing platform did not become a null state")
	}
}

func TestParseID(t *testing.T) {
	profile, platform, err := parseID("assistant:homeassistant")
	if err != nil || profile != "assistant" || platform != "homeassistant" {
		t.Fatalf("parseID = (%q, %q, %v)", profile, platform, err)
	}
	for _, id := range []string{"", "assistant", "Assistant:telegram", "assistant:home assistant"} {
		if _, _, err := parseID(id); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", id)
		}
	}
}

type fakeMessagingClient struct {
	platform   hermes.MessagingPlatform
	lastUpdate hermes.MessagingPlatformUpdateRequest
}

func (f *fakeMessagingClient) ListMessagingPlatforms(context.Context, string) ([]hermes.MessagingPlatform, error) {
	if f.platform.ID == "" {
		return nil, nil
	}
	return []hermes.MessagingPlatform{f.platform}, nil
}

func (f *fakeMessagingClient) UpdateMessagingPlatform(_ context.Context, _, platform string, update hermes.MessagingPlatformUpdateRequest) error {
	f.lastUpdate = update
	f.platform.ID = platform
	if update.Enabled != nil {
		f.platform.Enabled = *update.Enabled
	}
	f.platform.Configured = len(update.Env) > 0 || f.platform.Configured
	f.platform.State = "gateway_stopped"
	for key := range update.Env {
		f.platform.EnvVars = append(f.platform.EnvVars, hermes.MessagingEnvVar{Key: key, IsSet: true})
	}
	for _, key := range update.ClearEnv {
		for index := range f.platform.EnvVars {
			if f.platform.EnvVars[index].Key == key {
				f.platform.EnvVars[index].IsSet = false
			}
		}
	}
	return nil
}

func messagingState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), messagingTerraformValue(t, model)),
		Schema: schema,
	}
}

func messagingTerraformValue(t *testing.T, model resourceModel) map[string]tftypes.Value {
	t.Helper()
	return map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, terraformString(model.ID)),
		"profile":         tftypes.NewValue(tftypes.String, terraformString(model.Profile)),
		"platform":        tftypes.NewValue(tftypes.String, terraformString(model.Platform)),
		"enabled":         tftypes.NewValue(tftypes.Bool, terraformBool(model.Enabled)),
		"env_from":        terraformMap(t, model.EnvFrom),
		"configured":      tftypes.NewValue(tftypes.Bool, terraformBool(model.Configured)),
		"gateway_running": tftypes.NewValue(tftypes.Bool, terraformBool(model.GatewayRunning)),
		"state":           tftypes.NewValue(tftypes.String, terraformString(model.State)),
		"error_code":      tftypes.NewValue(tftypes.String, terraformString(model.ErrorCode)),
		"updated_at":      tftypes.NewValue(tftypes.String, terraformString(model.UpdatedAt)),
		"configured_keys": terraformList(t, model.ConfiguredKeys),
	}
}

func terraformMap(t *testing.T, value types.Map) tftypes.Value {
	t.Helper()
	terraformValue, err := value.ToTerraformValue(context.Background())
	if err != nil {
		t.Fatalf("convert map to Terraform value: %v", err)
	}
	return terraformValue
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
