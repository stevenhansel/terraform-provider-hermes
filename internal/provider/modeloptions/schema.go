package modeloptions

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func dataSourceSchema(_ context.Context, _ datasource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Reads Hermes model providers and their available models.",
		MarkdownDescription: "Reads the profile-scoped, non-secret model inventory from Hermes. " +
			"Use `refresh` sparingly because Hermes may contact provider catalogs when refreshing.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional Hermes profile to scope model discovery.",
				MarkdownDescription: "Optional Hermes profile to scope model discovery. An omitted value uses Hermes' active/default profile.",
			},
			"refresh": schema.BoolAttribute{
				Optional:            true,
				Description:         "Whether Hermes should refresh provider model catalogs instead of using its cache.",
				MarkdownDescription: "Whether Hermes should refresh provider model catalogs instead of using its cache. This can perform network requests to provider endpoints.",
			},
			"include_unconfigured": schema.BoolAttribute{
				Optional:            true,
				Description:         "Whether to include providers without configured credentials.",
				MarkdownDescription: "Whether to include providers without configured credentials.",
			},
			"explicit_only": schema.BoolAttribute{
				Optional:            true,
				Description:         "Whether to return only explicitly configured providers.",
				MarkdownDescription: "Whether to return only explicitly configured providers.",
			},
			"current_model": schema.StringAttribute{
				Computed: true,
			},
			"current_provider": schema.StringAttribute{
				Computed: true,
			},
			"providers": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"slug": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"models": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
						"total_models": schema.Int64Attribute{
							Computed: true,
						},
						"is_current": schema.BoolAttribute{
							Computed: true,
						},
						"is_user_defined": schema.BoolAttribute{
							Computed: true,
						},
						"source": schema.StringAttribute{
							Computed: true,
						},
						"authenticated": schema.BoolAttribute{
							Computed: true,
						},
						"auth_type": schema.StringAttribute{
							Computed: true,
						},
						"key_env": schema.StringAttribute{
							Computed: true,
						},
						"warning": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				Description:         "Available Hermes provider rows and their model identifiers.",
				MarkdownDescription: "Available Hermes provider rows and their model identifiers. Credentials and tokens are not returned.",
			},
		},
	}
}
