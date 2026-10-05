package skill

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages the enabled state of one existing Hermes skill.",
		MarkdownDescription: "Manages the enabled state of one existing Hermes skill. " +
			"This resource never creates or edits skill files; use `hermes_skill_installation` for explicit hub installation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the skill.",
				MarkdownDescription: "Lowercase Hermes profile that owns the skill. Use `default` explicitly when intended.",
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Hermes skill name, including a qualified `plugin:skill` or `category/name` name when applicable.",
				MarkdownDescription: "Hermes skill name, including a qualified `plugin:skill` or `category/name` name when applicable.",
			},
			"enabled": schema.BoolAttribute{
				Required:            true,
				Description:         "Whether Hermes should include this skill in its active skill inventory.",
				MarkdownDescription: "Whether Hermes should include this skill in its active skill inventory.",
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"category": schema.StringAttribute{
				Computed: true,
			},
			"usage": schema.Int64Attribute{
				Computed: true,
			},
			"provenance": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
