package cron

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

var cronProfilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var cronJobIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type resourceModel struct {
	ID              types.String `tfsdk:"id"`
	Profile         types.String `tfsdk:"profile"`
	Name            types.String `tfsdk:"name"`
	Prompt          types.String `tfsdk:"prompt"`
	Schedule        types.String `tfsdk:"schedule"`
	Deliver         types.String `tfsdk:"deliver"`
	Skills          types.List   `tfsdk:"skills"`
	Model           types.String `tfsdk:"model"`
	ModelProvider   types.String `tfsdk:"model_provider"`
	BaseURL         types.String `tfsdk:"base_url"`
	ContextFrom     types.List   `tfsdk:"context_from"`
	EnabledToolsets types.List   `tfsdk:"enabled_toolsets"`
	Workdir         types.String `tfsdk:"workdir"`
	Paused          types.Bool   `tfsdk:"paused"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	State           types.String `tfsdk:"state"`
	NextRunAt       types.String `tfsdk:"next_run_at"`
	LastRunAt       types.String `tfsdk:"last_run_at"`
	LastStatus      types.String `tfsdk:"last_status"`
}

// Resource manages a profile-scoped Hermes cron job without exposing
// arbitrary script execution through normal Terraform plans.
type Resource struct {
	client cronClient
}

// NewResource returns the Hermes cron job resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_cron_job"
}

func (r *Resource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = resourceSchema(ctx, request)
}

func (r *Resource) ValidateConfig(ctx context.Context, request resource.ValidateConfigRequest, response *resource.ValidateConfigResponse) {
	var config resourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateConfig(ctx, config); err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(cronClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes cron client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, jobID, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), profile+":"+jobID)...)
}

func (r *Resource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	if !r.ensureClient(&response.Diagnostics) {
		return
	}
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateConfig(ctx, plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job", err.Error())
		return
	}
	created, err := r.client.CreateCronJob(ctx, stringValue(plan.Profile), requestFromModel(ctx, plan, &response.Diagnostics))
	if err != nil {
		response.Diagnostics.AddError("Unable to create Hermes cron job", err.Error())
		return
	}
	if response.Diagnostics.HasError() {
		return
	}
	if strings.TrimSpace(created.ID) == "" {
		response.Diagnostics.AddError("Hermes returned an invalid cron job", "The create response did not contain a job ID.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, stringValue(plan.Profile), created.ID, &response.State, &response.Diagnostics)
}

func (r *Resource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	if !r.ensureClient(&response.Diagnostics) {
		return
	}
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	profile, jobID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job state", err.Error())
		return
	}
	r.readIntoState(ctx, profile, jobID, &response.State, &response.Diagnostics)
}

func (r *Resource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	if !r.ensureClient(&response.Diagnostics) {
		return
	}
	var plan resourceModel
	var state resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateConfig(ctx, plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job", err.Error())
		return
	}
	profile, jobID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job state", err.Error())
		return
	}
	updates, err := buildUpdates(ctx, plan, state)
	if err != nil {
		response.Diagnostics.AddError("Unable to prepare Hermes cron job update", err.Error())
		return
	}
	if len(updates) > 0 {
		if _, err := r.client.UpdateCronJob(ctx, profile, jobID, updates); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes cron job", err.Error())
			return
		}
	}
	if !plan.Paused.IsNull() && !plan.Paused.IsUnknown() && !state.Paused.IsNull() && !state.Paused.IsUnknown() && plan.Paused.ValueBool() != state.Paused.ValueBool() {
		if plan.Paused.ValueBool() {
			if _, err := r.client.PauseCronJob(ctx, profile, jobID); err != nil {
				response.Diagnostics.AddError("Unable to pause Hermes cron job", err.Error())
				return
			}
		} else if _, err := r.client.ResumeCronJob(ctx, profile, jobID); err != nil {
			response.Diagnostics.AddError("Unable to resume Hermes cron job", err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, profile, jobID, &response.State, &response.Diagnostics)
}

func (r *Resource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	if !r.ensureClient(&response.Diagnostics) {
		return
	}
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	profile, jobID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes cron job state", err.Error())
		return
	}
	if err := r.client.DeleteCronJob(ctx, profile, jobID); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete Hermes cron job", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing cron jobs.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, jobID string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	job, err := r.client.GetCronJob(ctx, profile, jobID)
	if err != nil {
		if hermes.IsNotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diagnostics.AddError("Unable to read Hermes cron job", err.Error())
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(ctx, &current, profile, job, diagnostics)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(ctx context.Context, config resourceModel) error {
	if config.Profile.IsUnknown() || config.Schedule.IsUnknown() || config.Deliver.IsUnknown() || config.Prompt.IsUnknown() || config.Skills.IsUnknown() {
		return nil
	}
	profile := stringValue(config.Profile)
	if profile == "" || !cronProfilePattern.MatchString(profile) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	if strings.TrimSpace(stringValue(config.Schedule)) == "" {
		return fmt.Errorf("schedule is required")
	}
	if strings.TrimSpace(stringValue(config.Deliver)) == "" {
		return fmt.Errorf("deliver must not be empty")
	}
	skills, err := listStrings(ctx, config.Skills)
	if err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	if strings.TrimSpace(stringValue(config.Prompt)) == "" && len(skills) == 0 {
		return fmt.Errorf("at least one of prompt or skills is required")
	}
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() && stringValue(config.BaseURL) != "" {
		parsed, err := url.Parse(stringValue(config.BaseURL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("base_url must be an absolute http or https URL")
		}
	}
	return nil
}

func requestFromModel(ctx context.Context, model resourceModel, diagnostics *diag.Diagnostics) hermes.CronJobRequest {
	skills, err := listStrings(ctx, model.Skills)
	if err != nil {
		diagnostics.AddError("Invalid Hermes cron skills", err.Error())
	}
	contextFrom, err := listStrings(ctx, model.ContextFrom)
	if err != nil {
		diagnostics.AddError("Invalid Hermes cron context_from", err.Error())
	}
	toolsets, err := listStrings(ctx, model.EnabledToolsets)
	if err != nil {
		diagnostics.AddError("Invalid Hermes cron enabled_toolsets", err.Error())
	}
	return hermes.CronJobRequest{
		Paused:          boolValue(model.Paused),
		Prompt:          stringValue(model.Prompt),
		Schedule:        stringValue(model.Schedule),
		Name:            stringValue(model.Name),
		Deliver:         stringValue(model.Deliver),
		Skills:          skills,
		Model:           stringValue(model.Model),
		Provider:        stringValue(model.ModelProvider),
		BaseURL:         stringValue(model.BaseURL),
		ContextFrom:     contextFrom,
		EnabledToolsets: toolsets,
		Workdir:         stringValue(model.Workdir),
	}
}

func buildUpdates(ctx context.Context, plan, state resourceModel) (map[string]any, error) {
	updates := make(map[string]any)
	if stringChanged(plan.Name, state.Name) && !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		updates["name"] = stringValue(plan.Name)
	}
	if stringChanged(plan.Prompt, state.Prompt) {
		updates["prompt"] = stringValue(plan.Prompt)
	}
	if stringChanged(plan.Schedule, state.Schedule) {
		updates["schedule"] = stringValue(plan.Schedule)
	}
	if stringChanged(plan.Deliver, state.Deliver) {
		updates["deliver"] = stringValue(plan.Deliver)
	}
	if stringChanged(plan.Model, state.Model) {
		updates["model"] = nullableString(plan.Model)
	}
	if stringChanged(plan.ModelProvider, state.ModelProvider) {
		updates["provider"] = nullableString(plan.ModelProvider)
	}
	if stringChanged(plan.BaseURL, state.BaseURL) {
		updates["base_url"] = nullableString(plan.BaseURL)
	}
	if stringChanged(plan.Workdir, state.Workdir) {
		updates["workdir"] = nullableString(plan.Workdir)
	}
	for _, field := range []struct {
		name  string
		plan  types.List
		state types.List
	}{
		{name: "skills", plan: plan.Skills, state: state.Skills},
		{name: "context_from", plan: plan.ContextFrom, state: state.ContextFrom},
		{name: "enabled_toolsets", plan: plan.EnabledToolsets, state: state.EnabledToolsets},
	} {
		if listChanged(field.plan, field.state) {
			values, err := listStrings(ctx, field.plan)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", field.name, err)
			}
			updates[field.name] = values
		}
	}
	return updates, nil
}

func setRemoteState(ctx context.Context, state *resourceModel, profile string, job hermes.CronJob, diagnostics *diag.Diagnostics) {
	if strings.TrimSpace(job.Profile) != "" {
		profile = job.Profile
	}
	state.ID = types.StringValue(profile + ":" + job.ID)
	state.Profile = types.StringValue(profile)
	state.Name = types.StringValue(job.Name)
	state.Prompt = types.StringValue(job.Prompt)
	state.Schedule = types.StringValue(job.ScheduleText())
	state.Deliver = types.StringValue(job.DeliverText())
	state.Model = stringPointerValue(job.Model)
	state.ModelProvider = stringPointerValue(job.Provider)
	state.BaseURL = stringPointerValue(job.BaseURL)
	state.Workdir = stringPointerValue(job.Workdir)
	state.Paused = types.BoolValue(job.IsPaused())
	state.Enabled = types.BoolValue(job.Enabled)
	state.State = types.StringValue(job.State)
	state.NextRunAt = stringPointerValue(job.NextRunAt)
	state.LastRunAt = stringPointerValue(job.LastRunAt)
	state.LastStatus = stringPointerValue(job.LastStatus)

	values, valueDiagnostics := types.ListValueFrom(ctx, types.StringType, job.SkillNames())
	diagnostics.Append(valueDiagnostics...)
	state.Skills = values
	values, valueDiagnostics = types.ListValueFrom(ctx, types.StringType, job.ContextFrom)
	diagnostics.Append(valueDiagnostics...)
	state.ContextFrom = values
	values, valueDiagnostics = types.ListValueFrom(ctx, types.StringType, job.EnabledToolsets)
	diagnostics.Append(valueDiagnostics...)
	state.EnabledToolsets = values
}

func stateIdentity(state resourceModel) (profile, jobID string, err error) {
	profile = stringValue(state.Profile)
	jobID = stringValue(state.ID)
	if parsedProfile, parsedID, ok := strings.Cut(jobID, ":"); ok {
		if profile == "" {
			profile = parsedProfile
		}
		jobID = parsedID
	}
	if profile == "" || jobID == "" {
		return "", "", fmt.Errorf("state must contain a <profile>:<job_id> identity")
	}
	if !cronProfilePattern.MatchString(profile) || !cronJobIDPattern.MatchString(jobID) {
		return "", "", fmt.Errorf("state contains an invalid profile or job ID")
	}
	return profile, jobID, nil
}

func parseID(id string) (profile, jobID string, err error) {
	profile, jobID, ok := strings.Cut(id, ":")
	if !ok || profile == "" || jobID == "" || !cronProfilePattern.MatchString(profile) || !cronJobIDPattern.MatchString(jobID) {
		return "", "", fmt.Errorf("ID must be <profile>:<job_id>")
	}
	return profile, jobID, nil
}

func listStrings(ctx context.Context, value types.List) ([]string, error) {
	if value.IsNull() {
		return nil, nil
	}
	if value.IsUnknown() {
		return nil, nil
	}
	var result []string
	if diagnostics := value.ElementsAs(ctx, &result, false); diagnostics.HasError() {
		return nil, fmt.Errorf("must contain only strings: %s", diagnostics.Errors()[0].Detail())
	}
	return result, nil
}

func listChanged(plan, state types.List) bool {
	if plan.IsUnknown() {
		return false
	}
	if state.IsUnknown() {
		return false
	}
	if plan.IsNull() {
		return !state.IsNull() && len(state.Elements()) > 0
	}
	if state.IsNull() {
		return true
	}
	return !plan.Equal(state)
}

func stringChanged(plan, state types.String) bool {
	if plan.IsUnknown() {
		return false
	}
	if state.IsUnknown() {
		return false
	}
	if plan.IsNull() {
		return !state.IsNull()
	}
	return state.IsNull() || plan.ValueString() != state.ValueString()
}

func nullableString(value types.String) any {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return stringValue(value)
}

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}

func stringPointerValue(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}

func boolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}
