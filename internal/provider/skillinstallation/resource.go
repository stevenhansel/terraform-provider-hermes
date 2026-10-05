package skillinstallation

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

var (
	installationProfilePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	installationIdentifierPattern = regexp.MustCompile(`^[^\x00-\x1f\x7f\s]{1,512}$`)
)

const installationActionTimeout = 10 * time.Minute

type resourceModel struct {
	ID          types.String `tfsdk:"id"`
	Profile     types.String `tfsdk:"profile"`
	Identifier  types.String `tfsdk:"identifier"`
	Name        types.String `tfsdk:"name"`
	Installed   types.Bool   `tfsdk:"installed"`
	TrustLevel  types.String `tfsdk:"trust_level"`
	ScanVerdict types.String `tfsdk:"scan_verdict"`
	LastAction  types.String `tfsdk:"last_action"`
}

// Resource owns one entry in Hermes' skill-hub lock file. Installation and
// uninstallation are explicit background actions; refresh never starts one.
type Resource struct {
	client skillInstallationClient
}

// NewResource returns the Hermes skill-hub installation resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_skill_installation"
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
		response.Diagnostics.AddError("Invalid Hermes skill installation", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(skillInstallationClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes skill-installation client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, identifier, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill installation ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("identifier"), identifier)...)
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
		response.Diagnostics.AddError("Invalid Hermes skill installation", err.Error())
		return
	}
	profile, identifier := stringValue(plan.Profile), stringValue(plan.Identifier)
	sources, err := r.client.ListSkillHubSources(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes skill-hub state", err.Error())
		return
	}
	if installation, found := findInstallation(sources, identifier); found {
		setRemoteState(&plan, profile, identifier, installation)
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
		return
	}

	action, err := r.client.StartSkillInstall(ctx, profile, identifier)
	if err != nil {
		response.Diagnostics.AddError("Unable to start Hermes skill installation", err.Error())
		return
	}
	plan.LastAction = types.StringValue(action)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := r.client.WaitForAction(ctx, action, installationActionTimeout); err != nil {
		response.Diagnostics.AddError("Hermes skill installation failed", err.Error())
		return
	}
	r.readIntoState(ctx, profile, identifier, &response.State, &response.Diagnostics)
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
	profile, identifier, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill installation state", err.Error())
		return
	}
	r.readIntoState(ctx, profile, identifier, &response.State, &response.Diagnostics)
}

func (r *Resource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	if !r.ensureClient(&response.Diagnostics) {
		return
	}
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	profile, identifier, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill installation state", err.Error())
		return
	}
	// All installation inputs are replacement-required. This update path is
	// retained for framework compatibility and only refreshes durable state.
	r.readIntoState(ctx, profile, identifier, &response.State, &response.Diagnostics)
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
	profile, identifier, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes skill installation state", err.Error())
		return
	}
	sources, err := r.client.ListSkillHubSources(ctx, profile)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes skill-hub state before destroy", err.Error())
		return
	}
	installation, found := findInstallation(sources, identifier)
	if !found {
		response.State.RemoveResource(ctx)
		return
	}
	if strings.TrimSpace(installation.Name) == "" {
		response.Diagnostics.AddError("Hermes skill installation has no removable name", "The hub lock entry did not include the installed skill name.")
		return
	}
	action, err := r.client.StartSkillUninstall(ctx, profile, installation.Name)
	if err != nil {
		response.Diagnostics.AddError("Unable to start Hermes skill uninstallation", err.Error())
		return
	}
	if err := r.client.WaitForAction(ctx, action, installationActionTimeout); err != nil {
		response.Diagnostics.AddError("Hermes skill uninstallation failed", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing skill installations.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, identifier string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	sources, err := r.client.ListSkillHubSources(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes skill-hub state", err.Error())
		return
	}
	installation, found := findInstallation(sources, identifier)
	if !found {
		state.RemoveResource(ctx)
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(&current, profile, identifier, installation)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(config resourceModel) error {
	if config.Profile.IsUnknown() || config.Identifier.IsUnknown() {
		return nil
	}
	if !installationProfilePattern.MatchString(stringValue(config.Profile)) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	identifier := stringValue(config.Identifier)
	if !installationIdentifierPattern.MatchString(identifier) || strings.Contains(identifier, "..") {
		return fmt.Errorf("identifier must be a non-empty skill-hub identifier without traversal or whitespace")
	}
	return nil
}

func findInstallation(sources hermes.SkillHubSources, identifier string) (hermes.SkillHubInstallation, bool) {
	installation, ok := sources.Installed[identifier]
	if !ok {
		return hermes.SkillHubInstallation{}, false
	}
	if installation.Identifier == "" {
		installation.Identifier = identifier
	}
	return installation, true
}

func setRemoteState(state *resourceModel, profile, identifier string, installation hermes.SkillHubInstallation) {
	state.ID = types.StringValue(installationID(profile, identifier))
	state.Profile = types.StringValue(profile)
	state.Identifier = types.StringValue(identifier)
	state.Name = types.StringValue(installation.Name)
	state.Installed = types.BoolValue(true)
	state.TrustLevel = types.StringValue(installation.TrustLevel)
	state.ScanVerdict = types.StringValue(installation.ScanVerdict)
}

func stateIdentity(state resourceModel) (profile, identifier string, err error) {
	profile = stringValue(state.Profile)
	identifier = stringValue(state.Identifier)
	if profile == "" || identifier == "" {
		parsedProfile, parsedIdentifier, parseErr := parseID(stringValue(state.ID))
		if parseErr == nil {
			if profile == "" {
				profile = parsedProfile
			}
			if identifier == "" {
				identifier = parsedIdentifier
			}
		}
	}
	if err := validateConfig(resourceModel{Profile: types.StringValue(profile), Identifier: types.StringValue(identifier)}); err != nil {
		return "", "", err
	}
	return profile, identifier, nil
}

func installationID(profile, identifier string) string {
	return profile + ":" + url.QueryEscape(identifier)
}

func parseID(id string) (profile, identifier string, err error) {
	profile, encodedIdentifier, ok := strings.Cut(id, ":")
	if !ok || encodedIdentifier == "" || !installationProfilePattern.MatchString(profile) {
		return "", "", fmt.Errorf("ID must be <profile>:<url-escaped-identifier>")
	}
	identifier, err = url.QueryUnescape(encodedIdentifier)
	if err != nil {
		return "", "", fmt.Errorf("decode skill identifier in ID: %w", err)
	}
	if err := validateConfig(resourceModel{Profile: types.StringValue(profile), Identifier: types.StringValue(identifier)}); err != nil {
		return "", "", err
	}
	return profile, identifier, nil
}

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}
