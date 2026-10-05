package custom

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
	customProfilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	customIDPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	customIDSanitizer    = regexp.MustCompile(`[^a-z0-9_-]+`)
)

type resourceModel struct {
	ID             types.String `tfsdk:"id"`
	Profile        types.String `tfsdk:"profile"`
	Name           types.String `tfsdk:"name"`
	BaseURL        types.String `tfsdk:"base_url"`
	Model          types.String `tfsdk:"model"`
	DiscoverModels types.Bool   `tfsdk:"discover_models"`
	ContextLength  types.Int64  `tfsdk:"context_length"`
	Models         types.List   `tfsdk:"models"`
	HasAPIKey      types.Bool   `tfsdk:"has_api_key"`
	IsCurrent      types.Bool   `tfsdk:"is_current"`
	Source         types.String `tfsdk:"source"`
}

// Resource manages a named entry in Hermes' providers map.
type Resource struct {
	client customProviderClient
}

// NewResource returns the Hermes custom provider resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_custom_provider"
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
		response.Diagnostics.AddError("Invalid Hermes custom provider", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(customProviderClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes custom provider client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, endpointID, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes custom provider ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), profile+":"+endpointID)...)
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
		response.Diagnostics.AddError("Invalid Hermes custom provider", err.Error())
		return
	}
	endpointID := endpointID(stringValue(plan.Name))
	if err := r.client.UpsertCustomEndpoint(ctx, stringValue(plan.Profile), endpointRequest(endpointID, plan)); err != nil {
		response.Diagnostics.AddError("Unable to create Hermes custom provider", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, stringValue(plan.Profile), endpointID, &response.State, &response.Diagnostics)
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
	profile, endpointID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes custom provider state", err.Error())
		return
	}
	r.readIntoState(ctx, profile, endpointID, &response.State, &response.Diagnostics)
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
		response.Diagnostics.AddError("Invalid Hermes custom provider", err.Error())
		return
	}
	profile, endpointID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes custom provider state", err.Error())
		return
	}
	if err := r.client.UpsertCustomEndpoint(ctx, profile, endpointRequest(endpointID, plan)); err != nil {
		response.Diagnostics.AddError("Unable to update Hermes custom provider", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, profile, endpointID, &response.State, &response.Diagnostics)
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
	profile, endpointID, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes custom provider state", err.Error())
		return
	}
	if err := r.client.DeleteCustomEndpoint(ctx, profile, endpointID); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete Hermes custom provider", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing custom providers.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, endpointID string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	result, err := r.client.ListCustomEndpoints(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes custom providers", err.Error())
		return
	}
	endpoint, found := findEndpoint(result.Endpoints, endpointID)
	if !found || endpoint.Source != "providers" {
		state.RemoveResource(ctx)
		return
	}
	var current resourceModel
	diagnostics.Append(state.Get(ctx, &current)...)
	if diagnostics.HasError() {
		return
	}
	setRemoteState(ctx, &current, profile, endpoint, diagnostics)
	diagnostics.Append(state.Set(ctx, &current)...)
}

func validateConfig(config resourceModel) error {
	if config.Profile.IsUnknown() || config.Name.IsUnknown() || config.BaseURL.IsUnknown() || config.Model.IsUnknown() {
		return nil
	}
	profile := stringValue(config.Profile)
	if profile == "" || !customProfilePattern.MatchString(profile) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	if strings.TrimSpace(stringValue(config.Name)) == "" {
		return fmt.Errorf("name is required")
	}
	parsed, err := url.Parse(stringValue(config.BaseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("base_url must be an absolute http or https URL")
	}
	if strings.TrimSpace(stringValue(config.Model)) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

func endpointRequest(id string, model resourceModel) hermes.CustomEndpointRequest {
	return hermes.CustomEndpointRequest{
		ID: id, Name: stringValue(model.Name), BaseURL: strings.TrimRight(stringValue(model.BaseURL), "/"),
		Model: stringValue(model.Model), DiscoverModels: boolValue(model.DiscoverModels),
	}
}

func findEndpoint(endpoints []hermes.CustomEndpoint, id string) (hermes.CustomEndpoint, bool) {
	for _, endpoint := range endpoints {
		if endpoint.ID == id {
			return endpoint, true
		}
	}
	return hermes.CustomEndpoint{}, false
}

func setRemoteState(ctx context.Context, state *resourceModel, profile string, endpoint hermes.CustomEndpoint, diagnostics *diag.Diagnostics) {
	state.ID = types.StringValue(profile + ":" + endpoint.ID)
	state.Profile = types.StringValue(profile)
	state.Name = types.StringValue(endpoint.Name)
	state.BaseURL = types.StringValue(endpoint.BaseURL)
	state.Model = types.StringValue(endpoint.Model)
	state.DiscoverModels = types.BoolValue(endpoint.DiscoverModels)
	if endpoint.ContextLength == nil {
		state.ContextLength = types.Int64Null()
	} else {
		state.ContextLength = types.Int64Value(int64(*endpoint.ContextLength))
	}
	state.HasAPIKey = types.BoolValue(endpoint.HasAPIKey)
	state.IsCurrent = types.BoolValue(endpoint.IsCurrent)
	state.Source = types.StringValue(endpoint.Source)
	models, modelDiagnostics := types.ListValueFrom(ctx, types.StringType, endpoint.Models)
	diagnostics.Append(modelDiagnostics...)
	state.Models = models
}

func endpointID(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = customIDSanitizer.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")
	if name == "" {
		return "custom"
	}
	if len(name) > 64 {
		return name[:64]
	}
	return name
}

func stateIdentity(state resourceModel) (profile, id string, err error) {
	profile = stringValue(state.Profile)
	identity := stringValue(state.ID)
	if parsedProfile, parsedID, ok := strings.Cut(identity, ":"); ok {
		if profile == "" {
			profile = parsedProfile
		}
		identity = parsedID
	}
	if profile == "" || identity == "" || !customProfilePattern.MatchString(profile) || !customIDPattern.MatchString(identity) {
		return "", "", fmt.Errorf("state must contain a valid <profile>:<provider_id> identity")
	}
	return profile, identity, nil
}

func parseID(id string) (profile, endpointIDValue string, err error) {
	profile, endpointIDValue, ok := strings.Cut(id, ":")
	if !ok || !customProfilePattern.MatchString(profile) || !customIDPattern.MatchString(endpointIDValue) {
		return "", "", fmt.Errorf("ID must be <profile>:<provider_id>")
	}
	return profile, endpointIDValue, nil
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
