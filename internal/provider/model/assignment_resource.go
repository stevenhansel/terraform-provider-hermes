package model

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type modelAssignmentResource struct {
	client modelClient
}

type modelAssignmentModel struct {
	ID                    types.String `tfsdk:"id"`
	Scope                 types.String `tfsdk:"scope"`
	Task                  types.String `tfsdk:"task"`
	ModelProvider         types.String `tfsdk:"model_provider"`
	Model                 types.String `tfsdk:"model"`
	BaseURL               types.String `tfsdk:"base_url"`
	APIKey                types.String `tfsdk:"api_key"`
	ConfirmExpensiveModel types.Bool   `tfsdk:"confirm_expensive_model"`
	Profile               types.String `tfsdk:"profile"`
}

// NewAssignmentResource returns the Hermes model-assignment resource.
func NewAssignmentResource() resource.Resource {
	return &modelAssignmentResource{}
}

func (r *modelAssignmentResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hermes_model_assignment"
}

func (r *modelAssignmentResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = modelAssignmentSchema(ctx, request)
}

func (r *modelAssignmentResource) ValidateConfig(ctx context.Context, request resource.ValidateConfigRequest, response *resource.ValidateConfigResponse) {
	var config modelAssignmentModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateModel(&config); err != nil {
		response.Diagnostics.AddError("Invalid Hermes model assignment", err.Error())
	}
}

func (r *modelAssignmentResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
	if response.Diagnostics.HasError() {
		return
	}

	scope, task, profile, err := parseModelID(request.ID)
	if err != nil {
		response.Diagnostics.AddError("Invalid Hermes model assignment ID", err.Error())
		return
	}
	if scope != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("scope"), scope)...)
	}
	if task != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("task"), task)...)
	}
	if profile != "" {
		response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("profile"), profile)...)
	}
}

func (r *modelAssignmentResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	client, ok := request.ProviderData.(*hermes.Client)
	if !ok {
		response.Diagnostics.AddError("Unexpected Hermes provider data", fmt.Sprintf("Expected *hermes.Client, got %T.", request.ProviderData))
		return
	}
	r.client = client
}

func (r *modelAssignmentResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	if r.client == nil {
		response.Diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before managing model assignments.")
		return
	}
	var plan modelAssignmentModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateModel(&plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes model assignment", err.Error())
		return
	}
	if err := r.client.SetModel(ctx, assignmentFromModel(plan)); err != nil {
		response.Diagnostics.AddError("Unable to assign Hermes model", err.Error())
		return
	}
	setModelIdentity(&plan)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *modelAssignmentResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	if r.client == nil {
		response.Diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before reading model assignments.")
		return
	}
	var state modelAssignmentModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	config, err := r.client.GetConfig(ctx, optionalString(state.Profile))
	if err != nil {
		response.Diagnostics.AddError("Unable to read Hermes model assignment", err.Error())
		return
	}
	assignment, found := config.FindModelAssignment(optionalString(state.Scope), optionalString(state.Task))
	if !found {
		response.State.RemoveResource(ctx)
		return
	}
	state.Scope = types.StringValue(assignment.Scope)
	state.Task = stringValueOrNull(assignment.Task)
	state.ModelProvider = types.StringValue(assignment.Provider)
	state.Model = types.StringValue(assignment.Model)
	if assignment.BaseURLKnown {
		state.BaseURL = stringValueOrNull(assignment.BaseURL)
	}
	// Hermes deliberately does not return API keys from its config endpoint.
	// Preserve the sensitive value already held in state instead of erasing it.
	setModelIdentity(&state)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *modelAssignmentResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	if r.client == nil {
		response.Diagnostics.AddError("Hermes provider is not configured", "Configure the Hermes provider before updating model assignments.")
		return
	}
	var plan modelAssignmentModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validateModel(&plan); err != nil {
		response.Diagnostics.AddError("Invalid Hermes model assignment", err.Error())
		return
	}
	if err := r.client.SetModel(ctx, assignmentFromModel(plan)); err != nil {
		response.Diagnostics.AddError("Unable to update Hermes model assignment", err.Error())
		return
	}
	setModelIdentity(&plan)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *modelAssignmentResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	// Hermes has no safe "restore the previous model" operation. A destroy
	// therefore relinquishes Terraform ownership without changing the active
	// assistant. Terraform removes the resource from state after this method.
	response.State.RemoveResource(ctx)
}
