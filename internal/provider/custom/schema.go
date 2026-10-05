package custom

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema(_ context.Context, _ resource.SchemaRequest) schema.Schema {
	return schema.Schema{
		Description: "Manages a named, non-secret Hermes custom model provider.",
		MarkdownDescription: "Manages a named, non-secret Hermes custom model provider. " +
			"Credentials remain outside Terraform/OpenTofu state and should be injected into Hermes through an external secret manager.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Description:         "Lowercase Hermes profile that owns the provider entry. Use `default` explicitly when intended.",
				MarkdownDescription: "Lowercase Hermes profile that owns the provider entry. Use `default` explicitly when intended.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "Display name for the provider entry.",
				MarkdownDescription: "Display name for the provider entry. The stable remote key is derived from the name at creation.",
			},
			"base_url": schema.StringAttribute{
				Required:            true,
				Description:         "OpenAI-compatible endpoint base URL.",
				MarkdownDescription: "OpenAI-compatible endpoint base URL, such as `http://llm.example.com:8000/v1`.",
			},
			"model": schema.StringAttribute{
				Required:            true,
				Description:         "Default model identifier served by the endpoint.",
				MarkdownDescription: "Default model identifier served by the endpoint.",
			},
			"discover_models": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				Description:         "Whether Hermes may discover models from the endpoint.",
				MarkdownDescription: "Whether Hermes may discover models from the endpoint.",
			},
			"context_length": schema.Int64Attribute{
				Computed:            true,
				Description:         "Context length reported or configured by Hermes.",
				MarkdownDescription: "Context length reported or configured by Hermes. This is read-only until Hermes exposes a lossless clear/update contract.",
			},
			"models": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				Description:         "Models discovered or configured for this endpoint.",
				MarkdownDescription: "Models discovered or configured for this endpoint.",
			},
			"has_api_key": schema.BoolAttribute{
				Computed:            true,
				Description:         "Whether Hermes has a credential reference for this endpoint; the value is never returned.",
				MarkdownDescription: "Whether Hermes has a credential reference for this endpoint; the value is never returned.",
			},
			"is_current": schema.BoolAttribute{
				Computed: true,
			},
			"source": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}
