package model

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
)

func modelAssignmentSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description:         "Assigns Hermes' main or auxiliary model through the dashboard REST API.",
		MarkdownDescription: "Assigns Hermes' main or auxiliary model through the dashboard REST API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"scope": schema.StringAttribute{
				Optional:            true,
				Description:         "Assignment scope: main (default) or auxiliary.",
				MarkdownDescription: "Assignment scope: `main` (default) or `auxiliary`.",
			},
			"task": schema.StringAttribute{
				Optional:            true,
				Description:         "Auxiliary task name when scope is auxiliary.",
				MarkdownDescription: "Auxiliary task name when `scope = \"auxiliary\"`.",
			},
			"model_provider": schema.StringAttribute{
				Required:            true,
				Description:         "Hermes model backend, such as custom or openai.",
				MarkdownDescription: "Hermes model backend, such as `custom` or `openai`.",
			},
			"model": schema.StringAttribute{
				Required:            true,
				Description:         "Model identifier understood by the selected Hermes backend.",
				MarkdownDescription: "Model identifier understood by the selected Hermes backend.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				Description:         "OpenAI-compatible base URL for custom or local providers.",
				MarkdownDescription: "OpenAI-compatible base URL for custom or local providers.",
			},
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Optional provider API key. It is stored in Terraform/OpenTofu state; prefer Hermes secret references when available.",
				MarkdownDescription: "Optional provider API key. It is stored in Terraform/OpenTofu state; prefer Hermes secret references when available.",
			},
			"confirm_expensive_model": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				Description:         "Explicitly confirm Hermes' expensive-model warning when assigning this model.",
				MarkdownDescription: "Explicitly confirm Hermes' expensive-model warning when assigning this model. Hermes otherwise returns a plan-time apply error when confirmation is required.",
			},
			"profile": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional Hermes profile to configure.",
				MarkdownDescription: "Optional Hermes profile to configure.",
			},
		},
	}
}
