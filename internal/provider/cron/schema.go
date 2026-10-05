package cron

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages one profile-scoped Hermes cron job.",
		MarkdownDescription: "Manages one profile-scoped Hermes cron job. " +
			"The resource manages agent prompts and scheduler configuration; arbitrary shell scripts are deliberately excluded from this first contract.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the job. Use `default` explicitly when intended.",
				MarkdownDescription: "Lowercase Hermes profile that owns the job. Use `default` explicitly when intended.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Optional human-readable job name. Hermes derives one from the prompt when omitted.",
				MarkdownDescription: "Optional human-readable job name. Hermes derives one from the prompt when omitted.",
			},
			"prompt": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Prompt sent to the Hermes agent when the job runs.",
				MarkdownDescription: "Prompt sent to the Hermes agent when the job runs. At least `prompt` or one `skills` entry is required.",
			},
			"schedule": schema.StringAttribute{
				Required:            true,
				Description:         "Hermes schedule expression, for example `every 1h` or a cron expression.",
				MarkdownDescription: "Hermes schedule expression, for example `every 1h` or a cron expression.",
			},
			"deliver": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("local"),
				Description:         "Delivery target such as `local` or a configured Hermes messaging platform.",
				MarkdownDescription: "Delivery target such as `local` or a configured Hermes messaging platform. The target must exist in Hermes at runtime.",
			},
			"skills": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Description:         "Optional Hermes skills to invoke for the job.",
				MarkdownDescription: "Optional Hermes skills to invoke for the job.",
			},
			"model": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Optional model override for this job.",
				MarkdownDescription: "Optional model override for this job.",
			},
			"model_provider": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Optional model-provider override for this job.",
				MarkdownDescription: "Optional model-provider override for this job. The API field is `provider`.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Optional OpenAI-compatible base URL override for this job.",
				MarkdownDescription: "Optional OpenAI-compatible base URL override for this job.",
			},
			"context_from": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Description:         "Optional IDs of jobs whose latest output is included as context.",
				MarkdownDescription: "Optional IDs of jobs whose latest output is included as context.",
			},
			"enabled_toolsets": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Description:         "Optional Hermes toolsets enabled for this job.",
				MarkdownDescription: "Optional Hermes toolsets enabled for this job.",
			},
			"workdir": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Optional working directory for the job.",
				MarkdownDescription: "Optional working directory for the job.",
			},
			"paused": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				Description:         "Whether the scheduler should keep this job paused.",
				MarkdownDescription: "Whether the scheduler should keep this job paused. Pausing preserves the durable job definition.",
			},
			"enabled": schema.BoolAttribute{
				Computed: true,
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
			"next_run_at": schema.StringAttribute{
				Computed: true,
			},
			"last_run_at": schema.StringAttribute{
				Computed: true,
			},
			"last_status": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
