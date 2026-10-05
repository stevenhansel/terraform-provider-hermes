package modeloptions

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type dataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Profile             types.String `tfsdk:"profile"`
	Refresh             types.Bool   `tfsdk:"refresh"`
	IncludeUnconfigured types.Bool   `tfsdk:"include_unconfigured"`
	ExplicitOnly        types.Bool   `tfsdk:"explicit_only"`
	CurrentModel        types.String `tfsdk:"current_model"`
	CurrentProvider     types.String `tfsdk:"current_provider"`
	Providers           types.List   `tfsdk:"providers"`
}

// DataSource reads Hermes' model-picker inventory without mutating it.
type DataSource struct {
	client modelOptionsClient
}

// NewDataSource returns the model options data source.
func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "hermes_model_options"
}

func (d *DataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = dataSourceSchema(ctx, request)
}

func (d *DataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(modelOptionsClient)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Hermes provider data",
			fmt.Sprintf("Expected a Hermes model-options client, got %T.", request.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *DataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	if d.client == nil {
		response.Diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before reading model options.")
		return
	}

	var config dataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if config.Profile.IsUnknown() || config.Refresh.IsUnknown() || config.IncludeUnconfigured.IsUnknown() || config.ExplicitOnly.IsUnknown() {
		response.Diagnostics.AddError("Unknown Hermes model-options configuration", "profile and model-options flags must be known when reading hermes_model_options.")
		return
	}

	options, err := d.client.GetModelOptions(
		ctx,
		stringValue(config.Profile),
		boolValue(config.Refresh),
		boolValue(config.IncludeUnconfigured),
		boolValue(config.ExplicitOnly),
	)
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes model options", err.Error())
		return
	}

	config.ID = types.StringValue(modelOptionsID(stringValue(config.Profile)))
	config.CurrentModel = types.StringValue(options.Model)
	config.CurrentProvider = types.StringValue(options.Provider)
	setProviders(ctx, &config, options.Providers, &response.Diagnostics)
	response.Diagnostics.Append(response.State.Set(ctx, &config)...)
}

var providerAttributeTypes = map[string]attr.Type{
	"slug":            types.StringType,
	"name":            types.StringType,
	"models":          types.ListType{ElemType: types.StringType},
	"total_models":    types.Int64Type,
	"is_current":      types.BoolType,
	"is_user_defined": types.BoolType,
	"source":          types.StringType,
	"authenticated":   types.BoolType,
	"auth_type":       types.StringType,
	"key_env":         types.StringType,
	"warning":         types.StringType,
}

func setProviders(ctx context.Context, state *dataSourceModel, providers []hermes.ModelOptionProvider, diagnostics *diag.Diagnostics) {
	elements := make([]attr.Value, 0, len(providers))
	for _, provider := range providers {
		models, modelDiagnostics := types.ListValueFrom(ctx, types.StringType, provider.Models)
		diagnostics.Append(modelDiagnostics...)
		object, objectDiagnostics := types.ObjectValue(providerAttributeTypes, map[string]attr.Value{
			"slug":            types.StringValue(provider.Slug),
			"name":            types.StringValue(provider.Name),
			"models":          models,
			"total_models":    types.Int64Value(int64(provider.TotalModels)),
			"is_current":      types.BoolValue(provider.IsCurrent),
			"is_user_defined": types.BoolValue(provider.IsUserDefined),
			"source":          types.StringValue(provider.Source),
			"authenticated":   types.BoolValue(provider.Authenticated),
			"auth_type":       types.StringValue(provider.AuthType),
			"key_env":         types.StringValue(provider.KeyEnv),
			"warning":         types.StringValue(provider.Warning),
		})
		diagnostics.Append(objectDiagnostics...)
		elements = append(elements, object)
	}
	providerList, providerDiagnostics := types.ListValue(types.ObjectType{AttrTypes: providerAttributeTypes}, elements)
	diagnostics.Append(providerDiagnostics...)
	state.Providers = providerList
}

func modelOptionsID(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return "model-options"
	}
	return "model-options:" + profile
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
