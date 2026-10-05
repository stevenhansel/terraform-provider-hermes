package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
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
	profilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	mcpNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

type resourceModel struct {
	ID             types.String `tfsdk:"id"`
	Profile        types.String `tfsdk:"profile"`
	Name           types.String `tfsdk:"name"`
	URL            types.String `tfsdk:"url"`
	Auth           types.String `tfsdk:"auth"`
	BearerTokenEnv types.String `tfsdk:"bearer_token_env"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Transport      types.String `tfsdk:"transport"`
}

// Resource manages one profile-scoped HTTP/SSE MCP registration.
type Resource struct {
	client mcpClient
}

// NewResource returns the Hermes MCP server resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_mcp_server"
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
		response.Diagnostics.AddError("Invalid Hermes MCP server", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(mcpClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes MCP client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, name, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes MCP server ID", err.Error())
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
		response.Diagnostics.AddError("Invalid Hermes MCP server", err.Error())
		return
	}
	bearerToken, err := bearerTokenFromPlan(plan)
	if err != nil {
		response.Diagnostics.AddError("Unable to resolve Hermes MCP bearer token", err.Error())
		return
	}
	if err := r.client.CreateMCPServer(ctx, hermes.MCPServerRequest{
		Name:        stringValue(plan.Name),
		URL:         stringValue(plan.URL),
		Auth:        stringValue(plan.Auth),
		BearerToken: bearerToken,
		Profile:     stringValue(plan.Profile),
	}); err != nil {
		response.Diagnostics.AddError("Unable to create Hermes MCP server", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, stringValue(plan.Profile), stringValue(plan.Name), &response.State, &response.Diagnostics)
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
	r.readIntoState(ctx, stringValue(state.Profile), stringValue(state.Name), &response.State, &response.Diagnostics)
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
		response.Diagnostics.AddError("Invalid Hermes MCP server", err.Error())
		return
	}
	profile := stringValue(plan.Profile)
	name := stringValue(plan.Name)
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !state.Enabled.IsNull() && !state.Enabled.IsUnknown() && plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		if err := r.client.SetMCPServerEnabled(ctx, profile, name, plan.Enabled.ValueBool()); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes MCP server", err.Error())
			return
		}
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
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
	if err := r.client.DeleteMCPServer(ctx, stringValue(state.Profile), stringValue(state.Name)); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete Hermes MCP server", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing MCP servers.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, name string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	servers, err := r.client.ListMCPServers(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes MCP servers", err.Error())
		return
	}
	server, found := findServer(servers, name)
	if !found {
		state.RemoveResource(ctx)
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	if server.URL == "" || server.Transport != "http" {
		diagnostics.AddError(
			"Unsupported Hermes MCP transport",
			"hermes_mcp_server manages HTTP/SSE registrations only; stdio and malformed registrations must be managed outside this resource.",
		)
		return
	}
	if normalizeAuth(server.Auth) == "header" && stringValue(current.BearerTokenEnv) == "" {
		diagnostics.AddError(
			"Hermes MCP bearer token reference is missing",
			"The remote server uses header authentication, but Hermes does not return the secret or its environment-variable name. Configure bearer_token_env and recreate the resource, or manage this server outside Terraform/OpenTofu.",
		)
		return
	}
	setRemoteState(&current, profile, server)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(config resourceModel) error {
	if config.Profile.IsUnknown() || config.Name.IsUnknown() || config.URL.IsUnknown() || config.Auth.IsUnknown() || config.BearerTokenEnv.IsUnknown() {
		return nil
	}
	profile := stringValue(config.Profile)
	if profile == "" || !profilePattern.MatchString(profile) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	name := stringValue(config.Name)
	if name == "" || !mcpNamePattern.MatchString(name) {
		return fmt.Errorf("name must start with a letter or number and contain only letters, numbers, dots, underscores, or hyphens")
	}
	parsed, err := url.Parse(stringValue(config.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("url must be an absolute http or https URL")
	}
	auth := strings.ToLower(stringValue(config.Auth))
	bearerTokenEnv := stringValue(config.BearerTokenEnv)
	if auth != "none" && auth != "header" && auth != "oauth" {
		return fmt.Errorf("auth must be none, header, or oauth")
	}
	if auth == "header" && bearerTokenEnv == "" {
		return fmt.Errorf("bearer_token_env is required when auth is header")
	}
	if auth != "header" && bearerTokenEnv != "" {
		return fmt.Errorf("bearer_token_env is only valid when auth is header")
	}
	return nil
}

func bearerTokenFromPlan(plan resourceModel) (string, error) {
	if strings.ToLower(stringValue(plan.Auth)) != "header" {
		return "", nil
	}
	envName := stringValue(plan.BearerTokenEnv)
	if envName == "" {
		return "", fmt.Errorf("bearer_token_env is required when auth is header")
	}
	token, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("environment variable %q is not set or empty", envName)
	}
	return token, nil
}

func findServer(servers []hermes.MCPServer, name string) (hermes.MCPServer, bool) {
	for _, server := range servers {
		if server.Name == name {
			return server, true
		}
	}
	return hermes.MCPServer{}, false
}

func setRemoteState(state *resourceModel, profile string, server hermes.MCPServer) {
	state.ID = types.StringValue(profile + ":" + server.Name)
	state.Profile = types.StringValue(profile)
	state.Name = types.StringValue(server.Name)
	state.URL = types.StringValue(server.URL)
	state.Auth = types.StringValue(normalizeAuth(server.Auth))
	state.Enabled = types.BoolValue(server.Enabled)
	state.Transport = types.StringValue(server.Transport)
}

func normalizeAuth(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "none"
	}
	return value
}

func parseID(id string) (profile, name string, err error) {
	profile, name, ok := strings.Cut(id, ":")
	if !ok || profile == "" || name == "" {
		return "", "", fmt.Errorf("ID must be <profile>:<name>")
	}
	if !profilePattern.MatchString(profile) {
		return "", "", fmt.Errorf("invalid profile in ID %q", id)
	}
	if !mcpNamePattern.MatchString(name) {
		return "", "", fmt.Errorf("invalid server name in ID %q", id)
	}
	return profile, name, nil
}

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}
