package cron

import (
	"context"
	"encoding/json"
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
	for _, name := range []string{
		"id", "profile", "name", "prompt", "schedule", "deliver", "skills", "model", "model_provider",
		"base_url", "context_from", "enabled_toolsets", "workdir", "paused", "enabled", "state",
		"next_run_at", "last_run_at", "last_status",
	} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("schema is missing %q", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	skills, diagnostics := types.ListValueFrom(context.Background(), types.StringType, []string{"morning-brief"})
	if diagnostics.HasError() {
		t.Fatalf("skills value: %v", diagnostics)
	}
	valid := resourceModel{
		Profile:  types.StringValue("assistant"),
		Prompt:   types.StringValue("Check my tasks"),
		Schedule: types.StringValue("every 1h"),
		Deliver:  types.StringValue("local"),
		Skills:   skills,
	}
	if err := validateConfig(context.Background(), valid); err != nil {
		t.Fatalf("valid cron config: %v", err)
	}

	skillOnly := valid
	skillOnly.Prompt = types.StringValue("")
	if err := validateConfig(context.Background(), skillOnly); err != nil {
		t.Fatalf("skill-only cron config: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*resourceModel)
	}{
		{name: "invalid profile", mutate: func(model *resourceModel) { model.Profile = types.StringValue("Assistant") }},
		{name: "missing schedule", mutate: func(model *resourceModel) { model.Schedule = types.StringValue("") }},
		{name: "empty payload", mutate: func(model *resourceModel) {
			model.Prompt = types.StringValue("")
			model.Skills = types.ListNull(types.StringType)
		}},
		{name: "invalid base URL", mutate: func(model *resourceModel) { model.BaseURL = types.StringValue("llama:8080") }},
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

func TestResourceCreateUpdateAndReadLifecycle(t *testing.T) {
	client := &fakeCronClient{}
	instance := &Resource{client: client}
	skills, diagnostics := types.ListValueFrom(context.Background(), types.StringType, []string{"morning-brief"})
	if diagnostics.HasError() {
		t.Fatalf("skills value: %v", diagnostics)
	}
	plan := resourceModel{
		Profile: types.StringValue("assistant"),
		Name:    types.StringValue("task-check"), Prompt: types.StringValue("Check my tasks"),
		Schedule: types.StringValue("every 1h"), Deliver: types.StringValue("local"), Skills: skills,
		Model: types.StringValue("local-model"), ModelProvider: types.StringValue("custom"),
		Paused: types.BoolValue(false), ContextFrom: types.ListNull(types.StringType), EnabledToolsets: types.ListNull(types.StringType),
	}
	planState := cronState(t, plan)
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
	if created.ID.ValueString() != "assistant:job123" || created.Schedule.ValueString() != "every 1h" || created.Skills.Elements()[0].(types.String).ValueString() != "morning-brief" {
		t.Fatalf("created state = %#v, want decoded Hermes job", created)
	}

	updatedPlan := created
	updatedPlan.Prompt = types.StringValue("Updated prompt")
	updatedPlan.Paused = types.BoolValue(true)
	updatedPlanState := cronState(t, updatedPlan)
	updateResponse := frameworkresource.UpdateResponse{State: createResponse.State}
	instance.Update(context.Background(), frameworkresource.UpdateRequest{
		Plan: tfsdk.Plan(updatedPlanState), State: createResponse.State,
	}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatalf("Update diagnostics: %v", updateResponse.Diagnostics)
	}
	if client.job.Prompt != "Updated prompt" || !client.job.IsPaused() {
		t.Fatalf("updated job = %#v, want updated prompt and paused state", client.job)
	}

	readRequest := frameworkresource.ReadRequest{State: updateResponse.State}
	readResponse := frameworkresource.ReadResponse{State: readRequest.State}
	instance.Read(context.Background(), readRequest, &readResponse)
	if readResponse.Diagnostics.HasError() {
		t.Fatalf("Read diagnostics: %v", readResponse.Diagnostics)
	}
	var read resourceModel
	readResponse.Diagnostics.Append(readResponse.State.Get(context.Background(), &read)...)
	if readResponse.Diagnostics.HasError() || !read.Paused.ValueBool() || read.Prompt.ValueString() != "Updated prompt" {
		t.Fatalf("read state = %#v, diagnostics = %v", read, readResponse.Diagnostics)
	}
}

func TestBuildUpdates(t *testing.T) {
	skills, diagnostics := types.ListValueFrom(context.Background(), types.StringType, []string{"one", "two"})
	if diagnostics.HasError() {
		t.Fatalf("skills value: %v", diagnostics)
	}
	plan := resourceModel{
		Name: types.StringValue("new name"), Prompt: types.StringValue("new prompt"),
		Schedule: types.StringValue("every 2h"), Deliver: types.StringValue("local"), Skills: skills,
		Model: types.StringNull(), ModelProvider: types.StringValue("custom"),
		ContextFrom: types.ListNull(types.StringType), EnabledToolsets: types.ListNull(types.StringType),
	}
	state := resourceModel{
		Name: types.StringValue("old name"), Prompt: types.StringValue("old prompt"),
		Schedule: types.StringValue("every 1h"), Deliver: types.StringValue("local"),
		Model: types.StringValue("old-model"), ModelProvider: types.StringNull(),
		Skills: types.ListNull(types.StringType), ContextFrom: types.ListNull(types.StringType), EnabledToolsets: types.ListNull(types.StringType),
	}
	updates, err := buildUpdates(context.Background(), plan, state)
	if err != nil {
		t.Fatalf("buildUpdates: %v", err)
	}
	if updates["name"] != "new name" || updates["prompt"] != "new prompt" || updates["schedule"] != "every 2h" || updates["model"] != nil || updates["provider"] != "custom" {
		t.Fatalf("updates = %#v, want changed cron fields", updates)
	}
	if !reflect.DeepEqual(updates["skills"], []string{"one", "two"}) {
		t.Fatalf("skills update = %#v", updates["skills"])
	}
}

func TestParseID(t *testing.T) {
	profile, jobID, err := parseID("assistant:job123")
	if err != nil || profile != "assistant" || jobID != "job123" {
		t.Fatalf("parseID = (%q, %q, %v)", profile, jobID, err)
	}
	for _, id := range []string{"", "assistant", "Assistant:job123", "assistant:bad/id"} {
		if _, _, err := parseID(id); err == nil {
			t.Fatalf("parseID(%q) succeeded, want error", id)
		}
	}
}

type fakeCronClient struct {
	job hermes.CronJob
}

func (f *fakeCronClient) GetCronJob(context.Context, string, string) (hermes.CronJob, error) {
	return f.job, nil
}

func (f *fakeCronClient) CreateCronJob(_ context.Context, profile string, request hermes.CronJobRequest) (hermes.CronJob, error) {
	provider := request.Provider
	model := request.Model
	f.job = hermes.CronJob{
		ID: "job123", Profile: profile, Name: request.Name, Prompt: request.Prompt,
		Schedule: json.RawMessage(`{"display":"every 1h"}`), ScheduleDisplay: request.Schedule,
		Deliver: json.RawMessage(`"local"`), Skills: request.Skills, Model: &model, Provider: &provider,
		Enabled: !request.Paused, State: "scheduled",
	}
	if request.Paused {
		f.job.State = "paused"
	}
	return f.job, nil
}

func (f *fakeCronClient) UpdateCronJob(_ context.Context, _, _ string, updates map[string]any) (hermes.CronJob, error) {
	if value, ok := updates["name"].(string); ok {
		f.job.Name = value
	}
	if value, ok := updates["prompt"].(string); ok {
		f.job.Prompt = value
	}
	if value, ok := updates["schedule"].(string); ok {
		f.job.ScheduleDisplay = value
	}
	if value, ok := updates["skills"].([]string); ok {
		f.job.Skills = value
	}
	return f.job, nil
}

func (f *fakeCronClient) DeleteCronJob(context.Context, string, string) error {
	f.job = hermes.CronJob{}
	return nil
}

func (f *fakeCronClient) PauseCronJob(context.Context, string, string) (hermes.CronJob, error) {
	f.job.Enabled = false
	f.job.State = "paused"
	return f.job, nil
}

func (f *fakeCronClient) ResumeCronJob(context.Context, string, string) (hermes.CronJob, error) {
	f.job.Enabled = true
	f.job.State = "scheduled"
	return f.job, nil
}

func cronState(t *testing.T, model resourceModel) tfsdk.State {
	t.Helper()
	schema := resourceSchema(context.Background(), frameworkresource.SchemaRequest{})
	return tfsdk.State{
		Raw:    tftypes.NewValue(schema.Type().TerraformType(context.Background()), cronTerraformValue(t, model)),
		Schema: schema,
	}
}

func cronTerraformValue(t *testing.T, model resourceModel) map[string]tftypes.Value {
	t.Helper()
	return map[string]tftypes.Value{
		"id":               tftypes.NewValue(tftypes.String, terraformString(model.ID)),
		"profile":          tftypes.NewValue(tftypes.String, terraformString(model.Profile)),
		"name":             tftypes.NewValue(tftypes.String, terraformString(model.Name)),
		"prompt":           tftypes.NewValue(tftypes.String, terraformString(model.Prompt)),
		"schedule":         tftypes.NewValue(tftypes.String, terraformString(model.Schedule)),
		"deliver":          tftypes.NewValue(tftypes.String, terraformString(model.Deliver)),
		"skills":           terraformList(t, model.Skills),
		"model":            tftypes.NewValue(tftypes.String, terraformString(model.Model)),
		"model_provider":   tftypes.NewValue(tftypes.String, terraformString(model.ModelProvider)),
		"base_url":         tftypes.NewValue(tftypes.String, terraformString(model.BaseURL)),
		"context_from":     terraformList(t, model.ContextFrom),
		"enabled_toolsets": terraformList(t, model.EnabledToolsets),
		"workdir":          tftypes.NewValue(tftypes.String, terraformString(model.Workdir)),
		"paused":           tftypes.NewValue(tftypes.Bool, terraformBool(model.Paused)),
		"enabled":          tftypes.NewValue(tftypes.Bool, terraformBool(model.Enabled)),
		"state":            tftypes.NewValue(tftypes.String, terraformString(model.State)),
		"next_run_at":      tftypes.NewValue(tftypes.String, terraformString(model.NextRunAt)),
		"last_run_at":      tftypes.NewValue(tftypes.String, terraformString(model.LastRunAt)),
		"last_status":      tftypes.NewValue(tftypes.String, terraformString(model.LastStatus)),
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
