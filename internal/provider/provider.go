package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/custom"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/mcp"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/model"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/modeloptions"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/profile"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/status"
)

const (
	defaultEndpoint = "http://127.0.0.1:9119"
	envEndpoint     = "HERMES_DASHBOARD_URL"
	envUsername     = "HERMES_DASHBOARD_USERNAME"
	envPassword     = "HERMES_DASHBOARD_PASSWORD"
)

type Provider struct {
	version string
}

type providerConfig struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version}
	}
}

func (p *Provider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "hermes"
	response.Version = p.version
}

func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = providerSchema()
}

func (p *Provider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var config providerConfig
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	endpoint, ok := configuredString(config.Endpoint, envEndpoint, defaultEndpoint, &response.Diagnostics)
	if !ok {
		return
	}
	username, ok := configuredString(config.Username, envUsername, "", &response.Diagnostics)
	if !ok {
		return
	}
	password, ok := configuredString(config.Password, envPassword, "", &response.Diagnostics)
	if !ok {
		return
	}

	client, err := NewClientWithVersion(endpoint, username, password, p.version)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes provider configuration", err.Error())
		return
	}

	response.ResourceData = client
	response.DataSourceData = client
}

func (p *Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		model.NewAssignmentResource,
		profile.NewResource,
		mcp.NewResource,
		custom.NewResource,
	}
}

func (p *Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		status.NewDataSource,
		modeloptions.NewDataSource,
	}
}

func providerSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				Description:         "Hermes dashboard URL. The dashboard API, not the API-server listener, is used for management.",
				MarkdownDescription: "Hermes dashboard URL. The dashboard API, not the API-server listener, is used for management.",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				Description:         "Username for Hermes' optional machine-friendly basic dashboard provider.",
				MarkdownDescription: "Username for Hermes' optional machine-friendly basic dashboard provider.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Password for Hermes' basic dashboard provider. Prefer HERMES_DASHBOARD_PASSWORD.",
				MarkdownDescription: "Password for Hermes' basic dashboard provider. Prefer `HERMES_DASHBOARD_PASSWORD`.",
			},
		},
		Description:         "Manages Hermes dashboard configuration through its authenticated REST API.",
		MarkdownDescription: "Manages Hermes dashboard configuration through its authenticated REST API.",
	}
}

func configuredString(value types.String, envName, fallback string, diagnostics *diag.Diagnostics) (string, bool) {
	if value.IsUnknown() {
		diagnostics.AddError("Unknown provider configuration", fmt.Sprintf("%s must be known during provider configuration.", envName))
		return "", false
	}
	if !value.IsNull() && strings.TrimSpace(value.ValueString()) != "" {
		return value.ValueString(), true
	}
	if fromEnv := strings.TrimSpace(os.Getenv(envName)); fromEnv != "" {
		return fromEnv, true
	}
	return fallback, true
}
