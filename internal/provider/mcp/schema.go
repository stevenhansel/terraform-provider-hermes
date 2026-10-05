package mcp

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages one HTTP/SSE MCP server registration in a Hermes profile.",
		MarkdownDescription: "Manages one HTTP/SSE MCP server registration in a Hermes profile. " +
			"Stdio registrations remain outside this resource's host-process safety boundary.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the MCP server. Use `default` explicitly when intended.",
				MarkdownDescription: "Lowercase Hermes profile that owns the MCP server. Use `default` explicitly when intended.",
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Stable Hermes MCP server name.",
				MarkdownDescription: "Stable Hermes MCP server name. Names may contain letters, numbers, `.`, `_`, and `-`.",
			},
			"url": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "HTTP or SSE MCP endpoint URL.",
				MarkdownDescription: "HTTP or SSE MCP endpoint URL. Changing the endpoint replaces the registration.",
			},
			"auth": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Default:             stringdefault.StaticString("none"),
				Description:         "Hermes MCP authentication mode: `none`, `header`, or interactive `oauth`.",
				MarkdownDescription: "Hermes MCP authentication mode: `none`, `header`, or interactive `oauth`. `header` reads the bearer token from the environment variable named by `bearer_token_env` during create.",
			},
			"bearer_token_env": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Environment variable containing the bearer token used for header authentication. The token is never persisted in provider state.",
				MarkdownDescription: "Environment variable containing the bearer token used for header authentication. The token is read only during create and is never persisted in Terraform/OpenTofu state.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				Description:         "Whether Hermes should use this MCP server.",
				MarkdownDescription: "Whether Hermes should use this MCP server. Toggling this preserves the registration.",
			},
			"transport": schema.StringAttribute{
				Computed:            true,
				Description:         "Transport reported by Hermes; this resource always reports `http`.",
				MarkdownDescription: "Transport reported by Hermes; this resource always reports `http`.",
			},
		},
	}
}
