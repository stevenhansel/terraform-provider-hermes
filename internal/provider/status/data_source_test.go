package status

import (
	"context"
	"testing"

	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func TestStatusSchemaContract(t *testing.T) {
	instance := NewDataSource()
	var response frameworkdatasource.SchemaResponse
	instance.Schema(context.Background(), frameworkdatasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", response.Diagnostics)
	}
	if diagnostics := response.Schema.ValidateImplementation(context.Background()); diagnostics.HasError() {
		t.Fatalf("schema validation diagnostics: %v", diagnostics)
	}
	for _, name := range []string{"id", "profile", "version", "release_date", "gateway_running", "gateway_state", "active_agents", "active_sessions", "gateway_busy", "gateway_drainable", "auth_required", "auth_providers", "auth_flows", "overall"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("status schema is missing %q", name)
		}
	}
}

func TestSetStatus(t *testing.T) {
	state := dataSourceModel{Profile: types.StringValue("assistant")}
	status := hermes.Status{
		Version:        "0.21.0",
		GatewayRunning: true,
		GatewayState:   "running",
		ActiveAgents:   1,
		ActiveSessions: 2,
		AuthProviders:  []string{"basic", "oidc"},
		AuthFlows:      []string{"cookie"},
		Overall:        "ok",
	}
	var diags diag.Diagnostics
	setStatus(context.Background(), &state, status, &diags)
	if diags.HasError() || state.Version.ValueString() != "0.21.0" || !state.GatewayRunning.ValueBool() || state.ActiveSessions.ValueInt64() != 2 || state.Overall.ValueString() != "ok" {
		t.Fatalf("state = %#v, diagnostics = %v", state, diags)
	}
	if len(state.AuthProviders.Elements()) != 2 || len(state.AuthFlows.Elements()) != 1 {
		t.Fatalf("auth lists = %v / %v, want two providers and one flow", state.AuthProviders, state.AuthFlows)
	}
}
