package messaging

import (
	"context"
	"fmt"
	"os"
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

var (
	messagingProfilePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	messagingPlatformPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	environmentKeyPattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type resourceModel struct {
	ID             types.String `tfsdk:"id"`
	Profile        types.String `tfsdk:"profile"`
	Platform       types.String `tfsdk:"platform"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	EnvFrom        types.Map    `tfsdk:"env_from"`
	Configured     types.Bool   `tfsdk:"configured"`
	GatewayRunning types.Bool   `tfsdk:"gateway_running"`
	State          types.String `tfsdk:"state"`
	ErrorCode      types.String `tfsdk:"error_code"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	ConfiguredKeys types.List   `tfsdk:"configured_keys"`
}

// Resource manages one member of Hermes' fixed messaging catalog. It never
// attempts onboarding flows, connectivity tests, or gateway restarts during
// normal Terraform lifecycle operations.
type Resource struct {
	client messagingClient
}

// NewResource returns the Hermes messaging platform resource.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_messaging_platform"
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
		response.Diagnostics.AddError("Invalid Hermes messaging platform", err.Error())
	}
}

func (r *Resource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(messagingClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes messaging client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *Resource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	profile, platform, err := parseID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes messaging platform ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("platform"), platform)...)
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
		response.Diagnostics.AddError("Invalid Hermes messaging platform", err.Error())
		return
	}
	update, err := buildCreateUpdate(ctx, plan)
	if err != nil {
		response.Diagnostics.AddError("Unable to prepare Hermes messaging platform", err.Error())
		return
	}
	if err := r.client.UpdateMessagingPlatform(ctx, stringValue(plan.Profile), stringValue(plan.Platform), update); err != nil {
		response.Diagnostics.AddError("Unable to configure Hermes messaging platform", err.Error())
		return
	}
	// Create responses start with an empty state. Seed it with the plan so
	// readIntoState can preserve the environment-variable references that
	// Hermes intentionally does not return.
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, stringValue(plan.Profile), stringValue(plan.Platform), &response.State, &response.Diagnostics)
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
	profile, platform, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes messaging platform state", err.Error())
		return
	}
	r.readIntoState(ctx, profile, platform, &response.State, &response.Diagnostics)
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
		response.Diagnostics.AddError("Invalid Hermes messaging platform", err.Error())
		return
	}
	profile, platform, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes messaging platform state", err.Error())
		return
	}
	update, err := buildUpdate(ctx, plan, state)
	if err != nil {
		response.Diagnostics.AddError("Unable to prepare Hermes messaging platform update", err.Error())
		return
	}
	if !updateEmpty(update) {
		if err := r.client.UpdateMessagingPlatform(ctx, profile, platform, update); err != nil {
			response.Diagnostics.AddError("Unable to update Hermes messaging platform", err.Error())
			return
		}
	}
	// Update responses may also have an empty state. The plan contains the
	// reference-only env_from map needed to retain write-only credentials.
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	r.readIntoState(ctx, profile, platform, &response.State, &response.Diagnostics)
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
	profile, platform, err := stateIdentity(state)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes messaging platform state", err.Error())
		return
	}
	keys, err := mapKeys(ctx, state.EnvFrom)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes messaging platform state", err.Error())
		return
	}
	if err := r.client.UpdateMessagingPlatform(ctx, profile, platform, hermes.MessagingPlatformUpdateRequest{
		Enabled:  boolPointer(false),
		ClearEnv: keys,
	}); err != nil && !hermes.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to disable Hermes messaging platform", err.Error())
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *Resource) ensureClient(diagnostics *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing messaging platforms.")
	return false
}

func (r *Resource) readIntoState(ctx context.Context, profile, platform string, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	platforms, err := r.client.ListMessagingPlatforms(ctx, profile)
	if err != nil {
		diagnostics.AddError("Unable to read Hermes messaging platforms", err.Error())
		return
	}
	remote, found := findPlatform(platforms, platform)
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

func validateConfig(ctx context.Context, config resourceModel) error {
	if config.Profile.IsUnknown() || config.Platform.IsUnknown() || config.Enabled.IsUnknown() || config.EnvFrom.IsUnknown() {
		return nil
	}
	profile := stringValue(config.Profile)
	if !messagingProfilePattern.MatchString(profile) {
		return fmt.Errorf("profile must be a lowercase name with letters, numbers, underscores, or hyphens")
	}
	platform := stringValue(config.Platform)
	if !messagingPlatformPattern.MatchString(platform) {
		return fmt.Errorf("platform must be a lowercase Hermes platform ID")
	}
	if _, err := environmentReferences(ctx, config.EnvFrom); err != nil {
		return err
	}
	return nil
}

func buildCreateUpdate(ctx context.Context, plan resourceModel) (hermes.MessagingPlatformUpdateRequest, error) {
	values, err := resolveEnvironmentReferences(ctx, plan.EnvFrom)
	if err != nil {
		return hermes.MessagingPlatformUpdateRequest{}, err
	}
	return hermes.MessagingPlatformUpdateRequest{Enabled: boolPointer(boolValue(plan.Enabled)), Env: values}, nil
}

func buildUpdate(ctx context.Context, plan, state resourceModel) (hermes.MessagingPlatformUpdateRequest, error) {
	update := hermes.MessagingPlatformUpdateRequest{}
	if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() &&
		(state.Enabled.IsNull() || state.Enabled.IsUnknown() || plan.Enabled.ValueBool() != state.Enabled.ValueBool()) {
		update.Enabled = boolPointer(plan.Enabled.ValueBool())
	}

	desired, desiredErr := environmentReferences(ctx, plan.EnvFrom)
	if desiredErr != nil {
		return hermes.MessagingPlatformUpdateRequest{}, desiredErr
	}
	current, currentErr := environmentReferences(ctx, state.EnvFrom)
	if currentErr != nil {
		return hermes.MessagingPlatformUpdateRequest{}, fmt.Errorf("state env_from: %w", currentErr)
	}
	if !stringMapEqual(desired, current) {
		values, err := resolveEnvironmentReferences(ctx, plan.EnvFrom)
		if err != nil {
			return hermes.MessagingPlatformUpdateRequest{}, err
		}
		update.Env = values
		update.ClearEnv = differenceKeys(current, desired)
	}
	return update, nil
}

func updateEmpty(update hermes.MessagingPlatformUpdateRequest) bool {
	return update.Enabled == nil && len(update.Env) == 0 && len(update.ClearEnv) == 0
}

func environmentReferences(ctx context.Context, value types.Map) (map[string]string, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var references map[string]string
	if diagnostics := value.ElementsAs(ctx, &references, false); diagnostics.HasError() {
		return nil, fmt.Errorf("env_from must contain only string values")
	}
	for key, name := range references {
		if !environmentKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("env_from key %q is not a valid environment-variable name", key)
		}
		if name = strings.TrimSpace(name); !environmentKeyPattern.MatchString(name) {
			return nil, fmt.Errorf("env_from value for %q must be a valid provider environment-variable name", key)
		}
		references[key] = name
	}
	return references, nil
}

func resolveEnvironmentReferences(ctx context.Context, value types.Map) (map[string]string, error) {
	references, err := environmentReferences(ctx, value)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(references))
	for key, name := range references {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			return nil, fmt.Errorf("provider environment variable %q for Hermes key %q is not set", name, key)
		}
		values[key] = value
	}
	return values, nil
}

func mapKeys(ctx context.Context, value types.Map) ([]string, error) {
	references, err := environmentReferences(ctx, value)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(references))
	for key := range references {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func differenceKeys(current, desired map[string]string) []string {
	keys := make([]string, 0)
	for key := range current {
		if _, ok := desired[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func stringMapEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func findPlatform(platforms []hermes.MessagingPlatform, id string) (hermes.MessagingPlatform, bool) {
	for _, platform := range platforms {
		if platform.ID == id {
			return platform, true
		}
	}
	return hermes.MessagingPlatform{}, false
}

func setRemoteState(ctx context.Context, state *resourceModel, profile string, platform hermes.MessagingPlatform, diagnostics *diag.Diagnostics) {
	state.ID = types.StringValue(profile + ":" + platform.ID)
	state.Profile = types.StringValue(profile)
	state.Platform = types.StringValue(platform.ID)
	state.Enabled = types.BoolValue(platform.Enabled)
	state.Configured = types.BoolValue(platform.Configured)
	state.GatewayRunning = types.BoolValue(platform.GatewayRunning)
	state.State = types.StringValue(platform.State)
	state.ErrorCode = types.StringValue(platform.ErrorCode)
	if platform.UpdatedAt == nil {
		state.UpdatedAt = types.StringNull()
	} else {
		state.UpdatedAt = types.StringValue(*platform.UpdatedAt)
	}
	keys := make([]string, 0, len(platform.EnvVars))
	for _, env := range platform.EnvVars {
		if env.IsSet {
			keys = append(keys, env.Key)
		}
	}
	sort.Strings(keys)
	values, valueDiagnostics := types.ListValueFrom(ctx, types.StringType, keys)
	diagnostics.Append(valueDiagnostics...)
	state.ConfiguredKeys = values
}

func stateIdentity(state resourceModel) (profile, platform string, err error) {
	profile = stringValue(state.Profile)
	platform = stringValue(state.Platform)
	identity := stringValue(state.ID)
	if parsedProfile, parsedPlatform, ok := strings.Cut(identity, ":"); ok {
		if profile == "" {
			profile = parsedProfile
		}
		if platform == "" {
			platform = parsedPlatform
		}
	}
	if !messagingProfilePattern.MatchString(profile) || !messagingPlatformPattern.MatchString(platform) {
		return "", "", fmt.Errorf("state must contain a valid <profile>:<platform> identity")
	}
	return profile, platform, nil
}

func parseID(id string) (profile, platform string, err error) {
	profile, platform, ok := strings.Cut(id, ":")
	if !ok || !messagingProfilePattern.MatchString(profile) || !messagingPlatformPattern.MatchString(platform) {
		return "", "", fmt.Errorf("ID must be <profile>:<platform>")
	}
	return profile, platform, nil
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

func boolPointer(value bool) *bool {
	return &value
}
