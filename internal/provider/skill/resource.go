package skill

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

var (
	skillProfilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	skillNamePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}$`)
)

type resourceModel struct {
	ID          types.String `tfsdk:"id"`
	Profile     types.String `tfsdk:"profile"`
	Name        types.String `tfsdk:"name"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	Usage       types.Int64  `tfsdk:"usage"`
	Provenance  types.String `tfsdk:"provenance"`
}

// Resource manages activation of a skill already present in Hermes' local
// inventory. It does not write SKILL.md files or install arbitrary content.
type Resource struct {
	client skillClient
}

// NewResource returns the Hermes skill activation resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_skill"
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
		response.Diagnostics.AddError("Invalid Hermes skill", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(skillClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes skill client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, name, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill ID", err.Error())
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
		response.Diagnostics.AddError("Invalid Hermes skill", err.Error())
		return
	}
	profile, name := stringValue(plan.Profile), stringValue(plan.Name)
	skills, err := r.client.ListSkills(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes skills", err.Error())
		return
	}
	if _, found := findSkill(skills, name); !found {
		response.Diagnostics.AddError(
			"Hermes skill is not visible",
			"The skill was not returned by Hermes. It may be disabled already or not installed; use hermes_skill_installation for hub installation.",
		)
		return
	}
	if err := r.client.SetSkillEnabled(ctx, profile, name, boolValue(plan.Enabled)); err != nil {
		response.Diagnostics.AddError("Unable to update Hermes skill", err.Error())
		return
	}
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
		response.Diagnostics.AddError("Invalid Hermes skill state", err.Error())
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
		response.Diagnostics.AddError("Invalid Hermes skill", err.Error())
		return
	}
	profile, name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill state", err.Error())
		return
	}
	if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() &&
		(state.Enabled.IsNull() || state.Enabled.IsUnknown() || plan.Enabled.ValueBool() != state.Enabled.ValueBool()) {
		if err := r.client.SetSkillEnabled(ctx, profile, name, plan.Enabled.ValueBool()); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes skill", err.Error())
			return
		}
	}
	// Hermes omits disabled skills from its inventory. Seed the response
	// state with the planned toggle before Read so a successful disable is
	// retained rather than mistaken for remote deletion.
	state.Enabled = plan.Enabled
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
		response.Diagnostics.AddError("Invalid Hermes skill state", err.Error())
		return
	}
	// List first. Toggling a missing skill would create a disabled-name entry
	// in Hermes' config and would be an unexpected destroy side effect.
	skills, err := r.client.ListSkills(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes skills before destroy", err.Error())
		return
	}
	if _, found := findSkill(skills, name); found {
		if err := r.client.SetSkillEnabled(ctx, profile, name, false); err != nil {
			response.Diagnostics.AddError("Unable to disable Hermes skill", err.Error())
			return
		}
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing skills.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, name string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	skills, err := r.client.ListSkills(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes skills", err.Error())
		return
	}
	remote, found := findSkill(skills, name)
	if !found {
		var current resourceModel
		diagnostics.Append(state.Get(ctx, &current)...)
		if diagnostics.HasError() {
			return
		}
		// Hermes currently omits disabled skills from GET /api/skills. A
		// resource whose last known desired state is disabled is therefore
		// retained, while an enabled skill disappearing is treated as drift.
		if !current.Enabled.IsNull() && !current.Enabled.IsUnknown() && !current.Enabled.ValueBool() {
			return
		}
		state.RemoveResource(ctx)
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(&current, profile, remote)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(config resourceModel) error {
	if config.Profile.IsUnknown() || config.Name.IsUnknown() || config.Enabled.IsUnknown() {
		return nil
	}
	profile := stringValue(config.Profile)
	if !skillProfilePattern.MatchString(profile) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	name := stringValue(config.Name)
	if !skillNamePattern.MatchString(name) || strings.Contains(name, "..") || strings.ContainsAny(name, "\\\r\n\t ") {
		return fmt.Errorf("name must be a non-empty Hermes skill name without traversal or whitespace")
	}
	return nil
}

func findSkill(skills []hermes.Skill, name string) (hermes.Skill, bool) {
	for _, skill := range skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return hermes.Skill{}, false
}

func setRemoteState(state *resourceModel, profile string, skill hermes.Skill) {
	state.ID = types.StringValue(skillID(profile, skill.Name))
	state.Profile = types.StringValue(profile)
	state.Name = types.StringValue(skill.Name)
	state.Enabled = types.BoolValue(skill.Enabled)
	state.Description = types.StringValue(skill.Description)
	state.Category = types.StringValue(skill.Category)
	state.Usage = types.Int64Value(skill.Usage)
	state.Provenance = types.StringValue(skill.Provenance)
}

func stateIdentity(state resourceModel) (profile, name string, err error) {
	profile = stringValue(state.Profile)
	name = stringValue(state.Name)
	if parsedProfile, parsedName, parseErr := parseID(stringValue(state.ID)); parseErr == nil {
		if profile == "" {
			profile = parsedProfile
		}
		if name == "" {
			name = parsedName
		}
	}
	if err := validateConfig(resourceModel{Profile: types.StringValue(profile), Name: types.StringValue(name), Enabled: types.BoolValue(false)}); err != nil {
		return "", "", err
	}
	return profile, name, nil
}

func skillID(profile, name string) string {
	return profile + ":" + url.QueryEscape(name)
}

func parseID(id string) (profile, name string, err error) {
	profile, encodedName, ok := strings.Cut(id, ":")
	if !ok || encodedName == "" || !skillProfilePattern.MatchString(profile) {
		return "", "", fmt.Errorf("ID must be <profile>:<url-escaped-skill-name>")
	}
	name, err = url.QueryUnescape(encodedName)
	if err != nil {
		return "", "", fmt.Errorf("decode skill name in ID: %w", err)
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
