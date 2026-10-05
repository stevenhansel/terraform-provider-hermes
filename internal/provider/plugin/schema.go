package plugin

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages one Hermes agent-plugin installation and runtime state.",
		MarkdownDescription: "Manages one Hermes agent-plugin installation and runtime state. " +
			"Plugin installation can execute third-party code; use curated catalog entries or review custom sources before apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"identifier": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Custom Hermes plugin source, such as `owner/repository` or an HTTPS/SSH Git URL.",
				MarkdownDescription: "Custom Hermes plugin source, such as `owner/repository` or an HTTPS/SSH Git URL. Do not use local `file://` sources.",
			},
			"catalog_name": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Curated Hermes plugin catalog name.",
				MarkdownDescription: "Curated Hermes plugin catalog name. Hermes resolves and pins the catalog source server-side.",
			},
			"ref": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Optional full 40-character commit SHA for a custom source.",
				MarkdownDescription: "Optional full 40-character commit SHA for a custom source. Tags and branch names are intentionally not accepted.",
			},
			"force": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
				Description:         "Whether to accept caution-level Hermes plugin scan findings. Dangerous findings remain blocked upstream.",
				MarkdownDescription: "Whether to accept caution-level Hermes plugin scan findings. Dangerous findings remain blocked upstream; review this security-sensitive option carefully.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				Description:         "Whether Hermes should enable the plugin at runtime.",
				MarkdownDescription: "Whether Hermes should enable the plugin at runtime. Disabling preserves the installation.",
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"version": schema.StringAttribute{
				Computed: true,
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"source": schema.StringAttribute{
				Computed: true,
			},
			"runtime_status": schema.StringAttribute{
				Computed: true,
			},
			"has_dashboard_manifest": schema.BoolAttribute{
				Computed: true,
			},
			"can_remove": schema.BoolAttribute{
				Computed: true,
			},
			"can_update_git": schema.BoolAttribute{
				Computed: true,
			},
			"auth_required": schema.BoolAttribute{
				Computed: true,
			},
			"auth_command": schema.StringAttribute{
				Computed: true,
			},
			"user_hidden": schema.BoolAttribute{
				Computed: true,
			},
			"removed_reason": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
