package status

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type dataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Profile          types.String `tfsdk:"profile"`
	Version          types.String `tfsdk:"version"`
	ReleaseDate      types.String `tfsdk:"release_date"`
	GatewayRunning   types.Bool   `tfsdk:"gateway_running"`
	GatewayState     types.String `tfsdk:"gateway_state"`
	ActiveAgents     types.Int64  `tfsdk:"active_agents"`
	ActiveSessions   types.Int64  `tfsdk:"active_sessions"`
	GatewayBusy      types.Bool   `tfsdk:"gateway_busy"`
	GatewayDrainable types.Bool   `tfsdk:"gateway_drainable"`
	AuthRequired     types.Bool   `tfsdk:"auth_required"`
	AuthProviders    types.List   `tfsdk:"auth_providers"`
	AuthFlows        types.List   `tfsdk:"auth_flows"`
	Overall          types.String `tfsdk:"overall"`
}

type DataSource struct {
	client statusClient
}

// NewDataSource returns the Hermes status data source.
func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = "hermes_status"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description:         "Reads non-secret Hermes dashboard and gateway status.",
		MarkdownDescription: "Reads non-secret Hermes dashboard and gateway status. An optional `profile` scopes the status request.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional Hermes profile to scope the status request.",
				MarkdownDescription: "Optional Hermes profile to scope the status request.",
			},
			"version": schema.StringAttribute{
				Computed: true,
			},
			"release_date": schema.StringAttribute{
				Computed: true,
			},
			"gateway_running": schema.BoolAttribute{
				Computed: true,
			},
			"gateway_state": schema.StringAttribute{
				Computed: true,
			},
			"active_agents": schema.Int64Attribute{
				Computed: true,
			},
			"active_sessions": schema.Int64Attribute{
				Computed: true,
			},
			"gateway_busy": schema.BoolAttribute{
				Computed: true,
			},
			"gateway_drainable": schema.BoolAttribute{
				Computed: true,
			},
			"auth_required": schema.BoolAttribute{
				Computed: true,
			},
			"auth_providers": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"auth_flows": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"overall": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *DataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(statusClient)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected a Hermes status client, got %T.", request.ProviderData))
		return
	}
	d.client = client
}

func (d *DataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	if d.client == nil {
		response.Diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before reading Hermes status.")
		return
	}
	var config dataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if config.Profile.IsUnknown() {
		response.Diagnostics.AddError("Unknown Hermes status profile", "profile must be known when reading hermes_status.")
		return
	}

	status, err := d.client.GetStatus(ctx, optionalString(config.Profile))
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes status", err.Error())
		return
	}
	config.ID = types.StringValue(statusID(optionalString(config.Profile)))
	setStatus(ctx, &config, status, &response.Diagnostics)
	response.Diagnostics.Append(response.State.Set(ctx, &config)...)
}

func setStatus(ctx context.Context, state *dataSourceModel, status hermes.Status, diagnostics *diag.Diagnostics) {
	state.Version = types.StringValue(status.Version)
	state.ReleaseDate = types.StringValue(status.ReleaseDate)
	state.GatewayRunning = types.BoolValue(status.GatewayRunning)
	state.GatewayState = types.StringValue(status.GatewayState)
	state.ActiveAgents = types.Int64Value(int64(status.ActiveAgents))
	state.ActiveSessions = types.Int64Value(int64(status.ActiveSessions))
	state.GatewayBusy = types.BoolValue(status.GatewayBusy)
	state.GatewayDrainable = types.BoolValue(status.GatewayDrainable)
	state.AuthRequired = types.BoolValue(status.AuthRequired)
	state.Overall = types.StringValue(status.Overall)

	providers, providerDiagnostics := types.ListValueFrom(ctx, types.StringType, status.AuthProviders)
	diagnostics.Append(providerDiagnostics...)
	state.AuthProviders = providers
	flows, flowDiagnostics := types.ListValueFrom(ctx, types.StringType, status.AuthFlows)
	diagnostics.Append(flowDiagnostics...)
	state.AuthFlows = flows
}

func statusID(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return "status"
	}
	return "status:" + profile
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}
