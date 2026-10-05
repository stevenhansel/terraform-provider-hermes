package plugin

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
	pluginNamePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}$`)
	pluginCatalogPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	pluginSHA125Pattern  = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

type resourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Identifier           types.String `tfsdk:"identifier"`
	CatalogName          types.String `tfsdk:"catalog_name"`
	Ref                  types.String `tfsdk:"ref"`
	Force                types.Bool   `tfsdk:"force"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Name                 types.String `tfsdk:"name"`
	Version              types.String `tfsdk:"version"`
	Description          types.String `tfsdk:"description"`
	Source               types.String `tfsdk:"source"`
	RuntimeStatus        types.String `tfsdk:"runtime_status"`
	HasDashboardManifest types.Bool   `tfsdk:"has_dashboard_manifest"`
	CanRemove            types.Bool   `tfsdk:"can_remove"`
	CanUpdateGit         types.Bool   `tfsdk:"can_update_git"`
	AuthRequired         types.Bool   `tfsdk:"auth_required"`
	AuthCommand          types.String `tfsdk:"auth_command"`
	UserHidden           types.Bool   `tfsdk:"user_hidden"`
	RemovedReason        types.String `tfsdk:"removed_reason"`
}

// Resource owns a user-installed or explicitly selected Hermes agent plugin.
// Dashboard-only plugin discovery remains outside this resource because its
// static assets and backend code are not a generic Terraform object.
type Resource struct {
	client pluginClient
}

// NewResource returns the Hermes agent-plugin resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_plugin"
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
		response.Diagnostics.AddError("Invalid Hermes plugin", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(pluginClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes plugin client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	name, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes plugin ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
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
		response.Diagnostics.AddError("Invalid Hermes plugin", err.Error())
		return
	}
	result, err := r.client.InstallAgentPlugin(ctx, hermes.AgentPluginInstallRequest{
		Identifier:  stringValue(plan.Identifier),
		Force:       boolValue(plan.Force),
		Enable:      boolValue(plan.Enabled),
		CatalogName: stringValue(plan.CatalogName),
		Ref:         stringValue(plan.Ref),
	})
	if err != nil {
		response.Diagnostics.AddError("Unable to install Hermes plugin", err.Error())
		return
	}
	if strings.TrimSpace(result.PluginName) == "" {
		response.Diagnostics.AddError("Hermes plugin installation returned no name", "The install response did not identify the installed plugin.")
		return
	}
	plan.Name = types.StringValue(result.PluginName)
	plan.ID = types.StringValue(pluginID(result.PluginName))
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, result.PluginName, &response.State, &response.Diagnostics)
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
	name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes plugin state", err.Error())
		return
	}
	r.readIntoState(ctx, name, &response.State, &response.Diagnostics)
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
	name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes plugin state", err.Error())
		return
	}
	if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() &&
		(state.Enabled.IsNull() || state.Enabled.IsUnknown() || plan.Enabled.ValueBool() != state.Enabled.ValueBool()) {
		if _, err := r.client.SetAgentPluginEnabled(ctx, name, plan.Enabled.ValueBool()); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes plugin runtime state", err.Error())
			return
		}
		state.Enabled = plan.Enabled
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, name, &response.State, &response.Diagnostics)
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
	name, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes plugin state", err.Error())
		return
	}
	plugins, err := r.client.ListAgentPlugins(ctx)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes plugins before destroy", err.Error())
		return
	}
	remote, found := findPlugin(plugins, name)
	if !found {
		response.State.RemoveResource(ctx)
		return
	}
	if !remote.CanRemove {
		response.Diagnostics.AddError("Hermes plugin cannot be removed", fmt.Sprintf("Plugin %q is not a removable user installation.", name))
		return
	}
	if _, err := r.client.DeleteAgentPlugin(ctx, name); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to remove Hermes plugin", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing plugins.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, name string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	plugins, err := r.client.ListAgentPlugins(ctx)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes plugins", err.Error())
		return
	}
	remote, found := findPlugin(plugins, name)
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

func validateConfig(config resourceModel) error {
	if config.Identifier.IsUnknown() || config.CatalogName.IsUnknown() || config.Ref.IsUnknown() || config.Force.IsUnknown() || config.Enabled.IsUnknown() {
		return nil
	}
	identifier := stringValue(config.Identifier)
	catalogName := stringValue(config.CatalogName)
	if (identifier == "") == (catalogName == "") {
		return fmt.Errorf("exactly one of identifier or catalog_name must be set")
	}
	if identifier != "" {
		if !validPluginIdentifier(identifier) {
			return fmt.Errorf("identifier must be an owner/repository shorthand or an HTTPS/SSH Git source; local and insecure URL schemes are rejected")
		}
	}
	if catalogName != "" && !pluginCatalogPattern.MatchString(catalogName) {
		return fmt.Errorf("catalog_name must be a lowercase Hermes catalog identifier")
	}
	ref := stringValue(config.Ref)
	if ref != "" && !pluginSHA125Pattern.MatchString(ref) {
		return fmt.Errorf("ref must be a full 40-character hexadecimal commit SHA")
	}
	if identifier != "" && ref == "" {
		return fmt.Errorf("ref is required for custom plugin sources so the installation is deterministic")
	}
	if catalogName != "" && ref != "" {
		return fmt.Errorf("ref cannot be combined with catalog_name; the catalog pin is selected by Hermes")
	}
	return nil
}

func validPluginIdentifier(identifier string) bool {
	if len(identifier) > 1024 || strings.ContainsAny(identifier, "\x00\r\n\t ") || strings.Contains(identifier, "..") {
		return false
	}
	for _, prefix := range []string{"file://", "http://"} {
		if strings.HasPrefix(strings.ToLower(identifier), prefix) {
			return false
		}
	}
	if strings.HasPrefix(identifier, "https://") || strings.HasPrefix(identifier, "ssh://") || strings.HasPrefix(identifier, "git@") {
		return true
	}
	parts := strings.Split(strings.Trim(identifier, "/"), "/")
	return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
}

func findPlugin(plugins []hermes.AgentPlugin, name string) (hermes.AgentPlugin, bool) {
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin, true
		}
	}
	return hermes.AgentPlugin{}, false
}

func setRemoteState(state *resourceModel, plugin hermes.AgentPlugin) {
	state.ID = types.StringValue(pluginID(plugin.Name))
	state.Name = types.StringValue(plugin.Name)
	state.Version = types.StringValue(plugin.Version)
	state.Description = types.StringValue(plugin.Description)
	state.Source = types.StringValue(plugin.Source)
	state.RuntimeStatus = types.StringValue(plugin.RuntimeStatus)
	state.Enabled = types.BoolValue(plugin.RuntimeStatus == "enabled")
	state.HasDashboardManifest = types.BoolValue(plugin.HasDashboardManifest)
	state.CanRemove = types.BoolValue(plugin.CanRemove)
	state.CanUpdateGit = types.BoolValue(plugin.CanUpdateGit)
	state.AuthRequired = types.BoolValue(plugin.AuthRequired)
	state.AuthCommand = types.StringValue(plugin.AuthCommand)
	state.UserHidden = types.BoolValue(plugin.UserHidden)
	state.RemovedReason = types.StringValue(plugin.RemovedReason)
}

func stateIdentity(state resourceModel) (string, error) {
	name := stringValue(state.Name)
	if name == "" {
		name, _ = parseID(stringValue(state.ID))
	}
	if !pluginNamePattern.MatchString(name) || strings.Contains(name, "..") || strings.ContainsAny(name, "\\\r\n\t ") {
		return "", fmt.Errorf("state must contain a valid plugin name")
	}
	return name, nil
}

func pluginID(name string) string {
	return "plugin:" + url.QueryEscape(name)
}

func parseID(id string) (string, error) {
	if strings.HasPrefix(id, "plugin:") {
		encoded := strings.TrimPrefix(id, "plugin:")
		if encoded == "" {
			return "", fmt.Errorf("ID must contain a plugin name")
		}
		name, err := url.QueryUnescape(encoded)
		if err != nil {
			return "", fmt.Errorf("decode plugin name in ID: %w", err)
		}
		return name, nil
	}
	if id == "" {
		return "", fmt.Errorf("ID must be plugin:<url-escaped-name> or a plugin name")
	}
	return id, nil
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
