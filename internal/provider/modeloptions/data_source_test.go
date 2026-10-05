package modeloptions

import (
	"context"
	"testing"

	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

func TestSchemaContract(t *testing.T) {
	var response frameworkdatasource.SchemaResponse
	NewDataSource().Schema(context.Background(), frameworkdatasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", response.Diagnostics)
	}
	if diagnostics := response.Schema.ValidateImplementation(context.Background()); diagnostics.HasError() {
		t.Fatalf("schema validation diagnostics: %v", diagnostics)
	}
	for _, name := range []string{"id", "profile", "refresh", "include_unconfigured", "explicit_only", "current_model", "current_provider", "providers"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("model options schema is missing %q", name)
		}
	}
}

func TestSetProviders(t *testing.T) {
	state := dataSourceModel{}
	var diagnostics diag.Diagnostics
	setProviders(context.Background(), &state, []hermes.ModelOptionProvider{
		{
			Slug: "llamacpp", Name: "llama.cpp", Models: []string{"local-model"}, TotalModels: 1,
			IsCurrent: true, IsUserDefined: true, Source: "custom", Authenticated: true,
		},
	}, &diagnostics)
	if diagnostics.HasError() {
		t.Fatalf("setProviders diagnostics: %v", diagnostics)
	}
	if state.Providers.IsNull() || len(state.Providers.Elements()) != 1 {
		t.Fatalf("providers = %#v, want one provider", state.Providers)
	}
	if state.Providers.Elements()[0].Type(context.Background()).String() != (types.ObjectType{AttrTypes: providerAttributeTypes}).String() {
		t.Fatalf("provider element type = %s, want model option object", state.Providers.Elements()[0].Type(context.Background()))
	}
}

func TestModelOptionsID(t *testing.T) {
	if got := modelOptionsID(""); got != "model-options" {
		t.Fatalf("modelOptionsID(empty) = %q", got)
	}
	if got := modelOptionsID("assistant"); got != "model-options:assistant" {
		t.Fatalf("modelOptionsID(assistant) = %q", got)
	}
}
