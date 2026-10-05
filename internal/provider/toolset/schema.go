package toolset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages the enabled state of one Hermes built-in toolset.",
		MarkdownDescription: "Manages the enabled state of one Hermes built-in toolset. " +
			"Toolsets are not created or deleted; destroy disables the selected capability.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the toolset toggle.",
				MarkdownDescription: "Lowercase Hermes profile that owns the toolset toggle. Use `default` explicitly when intended.",
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Hermes built-in toolset identifier.",
				MarkdownDescription: "Hermes built-in toolset identifier, such as `web` or another value returned by the Hermes dashboard.",
			},
			"enabled": schema.BoolAttribute{
				Required:            true,
				Description:         "Whether Hermes should enable this toolset.",
				MarkdownDescription: "Whether Hermes should enable this toolset.",
			},
			"label": schema.StringAttribute{
				Computed: true,
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"platform": schema.StringAttribute{
				Computed: true,
			},
			"platform_label": schema.StringAttribute{
				Computed: true,
			},
			"available": schema.BoolAttribute{
				Computed: true,
			},
			"configured": schema.BoolAttribute{
				Computed: true,
			},
			"tools": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
			"post_setup_started": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
