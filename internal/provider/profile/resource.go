package profile

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type resourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	CloneFrom      types.String `tfsdk:"clone_from"`
	CloneAll       types.Bool   `tfsdk:"clone_all"`
	NoSkills       types.Bool   `tfsdk:"no_skills"`
	IsDefault      types.Bool   `tfsdk:"is_default"`
	Model          types.String `tfsdk:"model"`
	ModelProvider  types.String `tfsdk:"model_provider"`
	HasEnv         types.Bool   `tfsdk:"has_env"`
	SkillCount     types.Int64  `tfsdk:"skill_count"`
	GatewayRunning types.Bool   `tfsdk:"gateway_running"`
}

type Resource struct {
	client profileClient
}

// NewResource returns the Hermes named-profile resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_profile"
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
	if err := validateName(config.Name); err != nil {
		response.Diagnostics.AddError("Invalid Hermes profile", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(profileClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes profile client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if err := validateName(types.StringValue(request.ID)); err != nil {
		response.Diagnostics.AddError("Invalid Hermes profile ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("name"), request.ID)...)
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
	if err := validateName(plan.Name); err != nil {
		response.Diagnostics.AddError("Invalid Hermes profile", err.Error())
		return
	}
	if err := r.client.CreateProfile(ctx, createRequest(plan)); err != nil {
		response.Diagnostics.AddError("Unable to create Hermes profile", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, optionalString(plan.Name), &response.State, &response.Diagnostics)
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
	r.readIntoState(ctx, optionalString(state.Name), &response.State, &response.Diagnostics)
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
	if err := validateName(plan.Name); err != nil {
		response.Diagnostics.AddError("Invalid Hermes profile", err.Error())
		return
	}

	currentName := optionalString(state.Name)
	if currentName == "" {
		currentName = optionalString(plan.Name)
	}
	if currentName == "default" || strings.EqualFold(currentName, "default") {
		if currentName != optionalString(plan.Name) {
			response.Diagnostics.AddError("Cannot rename Hermes default profile", "The default profile is a protected namespace.")
			return
		}
	}
	if currentName != optionalString(plan.Name) {
		if err := r.client.RenameProfile(ctx, currentName, optionalString(plan.Name)); err != nil {
			response.Diagnostics.AddError("Unable to rename Hermes profile", err.Error())
			return
		}
		currentName = optionalString(plan.Name)
	}
	if optionalString(plan.Description) != optionalString(state.Description) {
		if err := r.client.UpdateProfileDescription(ctx, currentName, optionalString(plan.Description)); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes profile description", err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, currentName, &response.State, &response.Diagnostics)
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
	name := optionalString(state.Name)
	if strings.EqualFold(name, "default") {
		response.Diagnostics.AddError("Cannot delete Hermes default profile", "The default profile is protected by Hermes.")
		return
	}
	if err := r.client.DeleteProfile(ctx, name); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete Hermes profile", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing profiles.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, name string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	profiles, err := r.client.ListProfiles(ctx)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes profiles", err.Error())
		return
	}
	remote, found := findProfile(profiles, name)
	if !found {
		state.RemoveResource(ctx)
		return
	}

	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(&current, remote)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func createRequest(plan resourceModel) hermes.CreateProfileRequest {
	return hermes.CreateProfileRequest{
		Name:        optionalString(plan.Name),
		CloneFrom:   optionalString(plan.CloneFrom),
		CloneAll:    boolValue(plan.CloneAll),
		NoSkills:    boolValue(plan.NoSkills),
		Description: optionalString(plan.Description),
	}
}

func validateName(value types.String) error {
	if value.IsUnknown() {
		return nil
	}
	name := strings.TrimSpace(optionalString(value))
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if name == "default" {
		return fmt.Errorf("the default profile is managed by Hermes and cannot be represented as a named profile")
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("name must be lowercase")
	}
	if !profileNamePattern.MatchString(name) {
		return fmt.Errorf("name must start with a letter or number and contain only letters, numbers, underscores, or hyphens")
	}
	return nil
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}

func boolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}

func findProfile(profiles []hermes.Profile, name string) (hermes.Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return hermes.Profile{}, false
}

func setRemoteState(state *resourceModel, profile hermes.Profile) {
	state.ID = types.StringValue(profile.Name)
	state.Name = types.StringValue(profile.Name)
	state.Description = types.StringValue(profile.Description)
	state.IsDefault = types.BoolValue(profile.IsDefault)
	state.Model = stringPointerValue(profile.Model)
	state.ModelProvider = stringPointerValue(profile.Provider)
	state.HasEnv = types.BoolValue(profile.HasEnv)
	state.SkillCount = types.Int64Value(int64(profile.SkillCount))
	state.GatewayRunning = types.BoolValue(profile.GatewayRunning)
}

func stringPointerValue(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}
