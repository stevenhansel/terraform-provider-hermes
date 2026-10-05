package skill

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
	for _, name := range []string{"id", "profile", "name", "enabled", "description", "category", "usage", "provenance"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("skill schema is missing %q", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	valid := resourceModel{
		Profile: types.StringValue("assistant"),
		Name:    types.StringValue("plugin:calendar/read-events"),
		Enabled: types.BoolValue(true),
	}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid skill config: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*resourceModel)
	}{
		{name: "invalid profile", mutate: func(model *resourceModel) { model.Profile = types.StringValue("Assistant") }},
		{name: "empty name", mutate: func(model *resourceModel) { model.Name = types.StringValue("") }},
		{name: "traversal", mutate: func(model *resourceModel) { model.Name = types.StringValue("../skill") }},
		{name: "whitespace", mutate: func(model *resourceModel) { model.Name = types.StringValue("daily brief") }},
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

func TestSkillIDRoundTrip(t *testing.T) {
	profile := "assistant"
	name := "plugin:calendar/read-events"
	id := skillID(profile, name)
	if id != "assistant:plugin%3Acalendar%2Fread-events" {
		t.Fatalf("skillID = %q", id)
	}
	gotProfile, gotName, err := parseID(id)
	if err != nil || gotProfile != profile || gotName != name {
		t.Fatalf("parseID = (%q, %q, %v), want (%q, %q)", gotProfile, gotName, err, profile, name)
	}
	for _, invalid := range []string{"", "assistant", "Assistant:name", "assistant:", "assistant:../skill"} {
		if _, _, err := parseID(invalid); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", invalid)
		}
	}
}

func TestResourceCreateUpdateAndDisabledRead(t *testing.T) {
	client := &fakeSkillClient{skills: []hermes.Skill{{
		Name: "morning-brief", Description: "Morning summary", Category: "productivity", Enabled: true, Usage: 4, Provenance: "bundled",
	}}}
	instance := &Resource{client: client}
	plan := resourceModel{Profile: types.StringValue("assistant"), Name: types.StringValue("morning-brief"), Enabled: types.BoolValue(true)}
	state := skillState(t, plan)
	createResponse := frameworkresource.CreateResponse{State: state}
	instance.Create(context.Background(), frameworkresource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResponse)
	if createResponse.Diagnostics.HasError() {
		t.Fatalf("Create diagnostics: %v", createResponse.Diagnostics)
	}
	var created resourceModel
	createResponse.Diagnostics.Append(createResponse.State.Get(context.Background(), &created)...)
	if createResponse.Diagnostics.HasError() || created.ID.ValueString() != "assistant:morning-brief" || created.Usage.ValueInt64() != 4 {
		t.Fatalf("created state = %#v, diagnostics = %v", created, createResponse.Diagnostics)
	}

	updated := created
	updated.Enabled = types.BoolValue(false)
	updateState := skillState(t, updated)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{Plan: tfsdk.Plan(updateState), State: createResponse.State}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.lastEnabled == nil || *client.lastEnabled {
		t.Fatalf("last enabled = %#v, want false", client.lastEnabled)
	}
	var disabled resourceModel
	updateResponse.Diagnostics.Append(updateResponse.State.Get(context.Background(), &disabled)...)
	if updateResponse.Diagnostics.HasError() || disabled.Enabled.ValueBool() {
		t.Fatalf("disabled state = %#v, diagnostics = %v", disabled, updateResponse.Diagnostics)
	}
}

type fakeSkillClient struct {
	skills      []hermes.Skill
	lastEnabled *bool
}

func (f *fakeSkillClient) ListSkills(context.Context, string) ([]hermes.Skill, error) {
	visible := make([]hermes.Skill, 0, len(f.skills))
	for _, skill := range f.skills {
		if skill.Enabled {
			visible = append(visible, skill)
		}
	}
	return visible, nil
}

func (f *fakeSkillClient) SetSkillEnabled(_ context.Context, _ string, name string, enabled bool) error {
	f.lastEnabled = &enabled
	for index := range f.skills {
		if f.skills[index].Name == name {
			f.skills[index].Enabled = enabled
		}
	}
	return nil
}

func skillState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), skillTerraformValue(model)),
		Schema: schema,
	}
}

func skillTerraformValue(model resourceModel) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"id":          terraformString(model.ID),
		"profile":     terraformString(model.Profile),
		"name":        terraformString(model.Name),
		"enabled":     terraformBool(model.Enabled),
		"description": terraformString(model.Description),
		"category":    terraformString(model.Category),
		"usage":       terraformInt64(model.Usage),
		"provenance":  terraformString(model.Provenance),
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
