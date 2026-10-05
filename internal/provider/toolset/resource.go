package toolset

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

var toolsetProfilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var toolsetNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

type resourceModel struct {
	ID               types.String `tfsdk:"id"`
	Profile          types.String `tfsdk:"profile"`
	Name             types.String `tfsdk:"name"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Label            types.String `tfsdk:"label"`
	Description      types.String `tfsdk:"description"`
	Platform         types.String `tfsdk:"platform"`
	PlatformLabel    types.String `tfsdk:"platform_label"`
	Available        types.Bool   `tfsdk:"available"`
	Configured       types.Bool   `tfsdk:"configured"`
	Tools            types.List   `tfsdk:"tools"`
	PostSetupStarted types.String `tfsdk:"post_setup_started"`
}

// Resource manages one built-in Hermes toolset toggle. It deliberately does
// not own provider credentials, models, or post-setup executable artifacts.
type Resource struct {
	client toolsetClient
}

// NewResource returns the Hermes toolset resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_toolset"
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
	if err := validateConfig(config); err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(toolsetClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes toolset client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, name, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("name"), name)...)
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
	if err := validateConfig(plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset", err.Error())
		return
	}
	profile, name := stringValue(plan.Profile), stringValue(plan.Name)
	toolsets, err := r.client.ListToolsets(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes toolsets", err.Error())
		return
	}
	if _, found := findToolset(toolsets, name); !found {
		response.Diagnostics.AddError("Hermes toolset not found", fmt.Sprintf("Hermes did not return configurable toolset %q for profile %q.", name, profile))
		return
	}
	result, err := r.client.SetToolsetEnabled(ctx, profile, name, boolValue(plan.Enabled))
	if err != nil {
		response.Diagnostics.AddError("Unable to update Hermes toolset", err.Error())
		return
	}
	setActionResult(&plan, result)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, profile, name, &response.State, &response.Diagnostics)
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
	profile, name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset state", err.Error())
		return
	}
	r.readIntoState(ctx, profile, name, &response.State, &response.Diagnostics)
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
	if err := validateConfig(plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset", err.Error())
		return
	}
	profile, name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset state", err.Error())
		return
	}
	if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() &&
		(state.Enabled.IsNull() || state.Enabled.IsUnknown() || plan.Enabled.ValueBool() != state.Enabled.ValueBool()) {
		result, err := r.client.SetToolsetEnabled(ctx, profile, name, plan.Enabled.ValueBool())
		if err != nil {
			response.Diagnostics.AddError("Unable to update Hermes toolset", err.Error())
			return
		}
		setActionResult(&state, result)
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, profile, name, &response.State, &response.Diagnostics)
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
	profile, name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes toolset state", err.Error())
		return
	}
	toolsets, err := r.client.ListToolsets(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes toolsets before destroy", err.Error())
		return
	}
	if _, found := findToolset(toolsets, name); found {
		if _, err := r.client.SetToolsetEnabled(ctx, profile, name, false); err != nil {
			response.Diagnostics.AddError("Unable to disable Hermes toolset", err.Error())
			return
		}
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing toolsets.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, name string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	toolsets, err := r.client.ListToolsets(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes toolsets", err.Error())
		return
	}
	remote, found := findToolset(toolsets, name)
	if !found {
		state.RemoveResource(ctx)
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(ctx, &current, profile, remote, diagnostics)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(config resourceModel) error {
	if config.Profile.IsUnknown() || config.Name.IsUnknown() || config.Enabled.IsUnknown() {
		return nil
	}
	if !toolsetProfilePattern.MatchString(stringValue(config.Profile)) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	if !toolsetNamePattern.MatchString(stringValue(config.Name)) {
		return fmt.Errorf("name must be a lowercase Hermes toolset identifier")
	}
	return nil
}

func findToolset(toolsets []hermes.Toolset, name string) (hermes.Toolset, bool) {
	for _, toolset := range toolsets {
		if toolset.Name == name {
			return toolset, true
		}
	}
	return hermes.Toolset{}, false
}

func setRemoteState(ctx context.Context, state *resourceModel, profile string, toolset hermes.Toolset, diagnostics *diag.Diagnostics) {
	state.ID = types.StringValue(profile + ":" + toolset.Name)
	state.Profile = types.StringValue(profile)
	state.Name = types.StringValue(toolset.Name)
	state.Enabled = types.BoolValue(toolset.Enabled)
	state.Label = types.StringValue(toolset.Label)
	state.Description = types.StringValue(toolset.Description)
	state.Platform = types.StringValue(toolset.Platform)
	state.PlatformLabel = types.StringValue(toolset.PlatformLabel)
	state.Available = types.BoolValue(toolset.Available)
	state.Configured = types.BoolValue(toolset.Configured)
	tools := append([]string(nil), toolset.Tools...)
	sort.Strings(tools)
	values, valueDiagnostics := types.ListValueFrom(ctx, types.StringType, tools)
	diagnostics.Append(valueDiagnostics...)
	state.Tools = values
}

func setActionResult(state *resourceModel, result hermes.ToolsetToggleResult) {
	if result.PostSetupStarted == nil {
		state.PostSetupStarted = types.StringNull()
		return
	}
	state.PostSetupStarted = types.StringValue(*result.PostSetupStarted)
}

func stateIdentity(state resourceModel) (profile, name string, err error) {
	profile = stringValue(state.Profile)
	name = stringValue(state.Name)
	if profile == "" || name == "" {
		parsedProfile, parsedName, ok := strings.Cut(stringValue(state.ID), ":")
		if ok {
			if profile == "" {
				profile = parsedProfile
			}
			if name == "" {
				name = parsedName
			}
		}
	}
	if err := validateConfig(resourceModel{Profile: types.StringValue(profile), Name: types.StringValue(name), Enabled: types.BoolValue(false)}); err != nil {
		return "", "", err
	}
	return profile, name, nil
}

func parseID(id string) (profile, name string, err error) {
	profile, name, ok := strings.Cut(id, ":")
	if !ok {
		return "", "", fmt.Errorf("ID must be <profile>:<toolset>")
	}
	if err := validateConfig(resourceModel{Profile: types.StringValue(profile), Name: types.StringValue(name), Enabled: types.BoolValue(false)}); err != nil {
		return "", "", err
	}
	return profile, name, nil
}

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}

func boolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}
