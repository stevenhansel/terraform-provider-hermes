package profile

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description:         "Manages a named Hermes profile through the dashboard REST API.",
		MarkdownDescription: "Manages a named Hermes profile through the dashboard REST API. The `default` profile is protected and cannot be managed by this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "Lowercase Hermes profile name. The default profile is protected.",
				MarkdownDescription: "Lowercase Hermes profile name. The `default` profile is protected.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Description:         "Human-readable profile description.",
				MarkdownDescription: "Human-readable profile description.",
			},
			"clone_from": schema.StringAttribute{
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:         "Optional existing profile to clone during creation.",
				MarkdownDescription: "Optional existing profile to clone during creation. Changing this requires replacement.",
			},
			"clone_all": schema.BoolAttribute{
				Optional:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:         "Clone the source profile's complete state during creation.",
				MarkdownDescription: "Clone the source profile's complete state during creation. Changing this requires replacement.",
			},
			"no_skills": schema.BoolAttribute{
				Optional:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:         "Skip bundled skill seeding during creation.",
				MarkdownDescription: "Skip bundled skill seeding during creation. Changing this requires replacement.",
			},
			"is_default": schema.BoolAttribute{
				Computed: true,
			},
			"model": schema.StringAttribute{
				Computed:            true,
				Description:         "Currently selected main model, when reported by Hermes.",
				MarkdownDescription: "Currently selected main model, when reported by Hermes.",
			},
			"model_provider": schema.StringAttribute{
				Computed:            true,
				Description:         "Currently selected model provider, when reported by Hermes.",
				MarkdownDescription: "Currently selected model provider, when reported by Hermes.",
			},
			"has_env": schema.BoolAttribute{
				Computed: true,
			},
			"skill_count": schema.Int64Attribute{
				Computed: true,
			},
			"gateway_running": schema.BoolAttribute{
				Computed: true,
			},
		},
	}
}
