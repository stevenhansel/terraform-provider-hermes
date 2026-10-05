package messaging

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
		Description: "Manages one fixed Hermes messaging platform without storing credentials in state.",
		MarkdownDescription: "Manages one fixed Hermes messaging platform without storing credentials in Terraform/OpenTofu state. " +
			"Use `env_from` to name environment variables supplied to the provider process; values are sent to Hermes only during apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the platform. Use `default` explicitly when intended.",
				MarkdownDescription: "Lowercase Hermes profile that owns the platform. Use `default` explicitly when intended.",
			},
			"platform": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Hermes platform ID, such as `telegram`, `homeassistant`, or `email`.",
				MarkdownDescription: "Hermes platform ID from the fixed `/api/messaging/platforms` catalog, such as `telegram`, `homeassistant`, or `email`.",
			},
			"enabled": schema.BoolAttribute{
				Required:            true,
				Description:         "Whether Hermes should enable this platform.",
				MarkdownDescription: "Whether Hermes should enable this platform. Hermes may require a gateway restart before the change is active.",
			},
			"env_from": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				Description:         "Map of Hermes environment-variable keys to provider-process variable names.",
				MarkdownDescription: "Map of Hermes environment-variable keys to provider-process variable names. For example, `HASS_TOKEN = \"HERMES_HOMEASSISTANT_TOKEN\"`. Raw values are never stored in state.",
			},
			"configured": schema.BoolAttribute{
				Computed: true,
			},
			"gateway_running": schema.BoolAttribute{
				Computed: true,
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
			"error_code": schema.StringAttribute{
				Computed: true,
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},
			"configured_keys": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Description:         "Messaging environment-variable keys reported as set by Hermes; values are not returned.",
				MarkdownDescription: "Messaging environment-variable keys reported as set by Hermes; values are not returned.",
			},
		},
	}
}
