package profile

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func TestProfileSchemaContract(t *testing.T) {
	instance := NewResource()
	var response frameworkresource.SchemaResponse
	instance.Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", response.Diagnostics)
	}
	if diagnostics := response.Schema.ValidateImplementation(context.Background()); diagnostics.HasError() {
		t.Fatalf("schema validation diagnostics: %v", diagnostics)
	}
	for _, name := range []string{"id", "name", "description", "clone_from", "clone_all", "no_skills", "is_default", "model", "model_provider", "has_env", "skill_count", "gateway_running"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("profile schema is missing %q", name)
		}
	}
}

func TestValidateProfileName(t *testing.T) {
	for _, test := range []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "valid", value: "assistant"},
		{name: "hyphen", value: "personal-helper"},
		{name: "default protected", value: "default", wantError: true},
		{name: "uppercase", value: "Assistant", wantError: true},
		{name: "space", value: "personal helper", wantError: true},
		{name: "empty", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateName(types.StringValue(test.value))
			if (err != nil) != test.wantError {
				t.Fatalf("validateName(%q) error = %v, wantError = %t", test.value, err, test.wantError)
			}
		})
	}
}

func TestFindProfileAndRemoteState(t *testing.T) {
	model := &resourceModel{}
	remote := hermes.Profile{
		Name:           "assistant",
		IsDefault:      false,
		Description:    "Personal helper",
		HasEnv:         true,
		SkillCount:     4,
		GatewayRunning: true,
	}
	if found, ok := findProfile([]hermes.Profile{remote}, "assistant"); !ok || found.Name != "assistant" {
		t.Fatalf("findProfile() = %#v, %t; want assistant", found, ok)
	}
	if _, ok := findProfile([]hermes.Profile{remote}, "missing"); ok {
		t.Fatal("findProfile found a missing profile")
	}
	setRemoteState(model, remote)
	if model.ID.ValueString() != "assistant" || model.Description.ValueString() != "Personal helper" || !model.HasEnv.ValueBool() || model.SkillCount.ValueInt64() != 4 || !model.GatewayRunning.ValueBool() {
		t.Fatalf("remote state = %#v, want decoded profile", model)
	}
}

func TestResourceCreateAndReadLifecycle(t *testing.T) {
	client := &fakeProfileClient{}
	instance := &Resource{client: client}
	plan := resourceModel{
		Name:        types.StringValue("assistant"),
		Description: types.StringValue("Personal helper"),
		CloneFrom:   types.StringValue("default"),
		NoSkills:    types.BoolValue(true),
	}
	planState := profileState(t, plan)
	createRequest := frameworkresource.CreateRequest{Plan: tfsdk.Plan(planState)}
	createResponse := frameworkresource.CreateResponse{State: planState}
	instance.Create(context.Background(), createRequest, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	if client.created.Name != "assistant" || client.created.CloneFrom != "default" || !client.created.NoSkills {
		t.Fatalf("create request = %#v, want clone options", client.created)
	}

	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("decode Create state: %v", createResponse.Diagnostics)
	}
	if created.ID.ValueString() != "assistant" || created.Name.ValueString() != "assistant" || created.SkillCount.ValueInt64() != 3 {
		t.Fatalf("created state = %#v, want remote profile values", created)
	}

	readRequest := frameworkresource.ReadRequest{State: createResponse.State}
	readResponse := frameworkresource.ReadResponse{State: readRequest.State}
	instance.Read(context.Background(), readRequest, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", readResponse.Diagnostics)
	}
	var read resourceModel
	readResponse.Diagnostics.Append(readResponse.State.Get(context.Background(), &read)...)
	if readResponse.Diagnostics.HasError() || read.Description.ValueString() != "Personal helper" {
		t.Fatalf("read state = %#v, diagnostics = %v", read, readResponse.Diagnostics)
	}
}

type fakeProfileClient struct {
	profiles []hermes.Profile
	created  hermes.CreateProfileRequest
}

func (f *fakeProfileClient) ListProfiles(context.Context) ([]hermes.Profile, error) {
	if len(f.profiles) == 0 {
		f.profiles = []hermes.Profile{{Name: "assistant", Description: "Personal helper", SkillCount: 3}}
	}
	return f.profiles, nil
}

func (f *fakeProfileClient) CreateProfile(_ context.Context, request hermes.CreateProfileRequest) error {
	f.created = request
	f.profiles = []hermes.Profile{{Name: request.Name, Description: request.Description, SkillCount: 3}}
	return nil
}

func (f *fakeProfileClient) RenameProfile(_ context.Context, name, newName string) error {
	for index := range f.profiles {
		if f.profiles[index].Name == name {
			f.profiles[index].Name = newName
		}
	}
	return nil
}

func (f *fakeProfileClient) UpdateProfileDescription(_ context.Context, name, description string) error {
	for index := range f.profiles {
		if f.profiles[index].Name == name {
			f.profiles[index].Description = description
		}
	}
	return nil
}

func (f *fakeProfileClient) DeleteProfile(_ context.Context, name string) error {
	for index := range f.profiles {
		if f.profiles[index].Name == name {
			f.profiles = append(f.profiles[:index], f.profiles[index+1:]...)
			break
		}
	}
	return nil
}

func profileState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), profileTerraformValue(model)),
		Schema: schema,
	}
}

func profileTerraformValue(model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":              terraformString(model.ID),
		"name":            terraformString(model.Name),
		"description":     terraformString(model.Description),
		"clone_from":      terraformString(model.CloneFrom),
		"clone_all":       terraformBool(model.CloneAll),
		"no_skills":       terraformBool(model.NoSkills),
		"is_default":      terraformBool(model.IsDefault),
		"model":           terraformString(model.Model),
		"model_provider":  terraformString(model.ModelProvider),
		"has_env":         terraformBool(model.HasEnv),
		"skill_count":     terraformInt64(model.SkillCount),
		"gateway_running": terraformBool(model.GatewayRunning),
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

func terraformInt64(value types.Int64) tftypes.Value {
	if value.IsNull() {
		return tftypes.NewValue(tftypes.Number, nil)
	}
	return tftypes.NewValue(tftypes.Number, value.ValueInt64())
}
