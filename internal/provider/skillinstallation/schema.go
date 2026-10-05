package skillinstallation

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages one Hermes skill-hub installation with bounded background-action polling.",
		MarkdownDescription: "Manages one Hermes skill-hub installation with Hermes' security scan and bounded " +
			"background-action polling. It never accepts raw skill-file content.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the installed skill.",
				MarkdownDescription: "Lowercase Hermes profile that owns the installed skill. Use `default` explicitly when intended.",
			},
			"identifier": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Hermes skill-hub identifier, for example `official/productivity/morning-brief`.",
				MarkdownDescription: "Hermes skill-hub identifier, for example `official/productivity/morning-brief`.",
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"installed": schema.BoolAttribute{
				Computed: true,
			},
			"trust_level": schema.StringAttribute{
				Computed: true,
			},
			"scan_verdict": schema.StringAttribute{
				Computed: true,
			},
			"last_action": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
