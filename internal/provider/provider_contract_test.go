package provider

import (
	"context"
	"testing"

	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	frameworkproviderschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkresourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/cron"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/custom"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/mcp"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/messaging"
	"github.com/stevenhansel/terraform-provider-hermes/internal/provider/model"
)

func TestProviderMetadataAndResources(t *testing.T) {
	instance := &Provider{version: "test"}

	var metadata frameworkprovider.MetadataResponse
	instance.Metadata(context.Background(), frameworkprovider.MetadataRequest{}, &metadata)
	if metadata.TypeName != "hermes" || metadata.Version != "test" {
		t.Fatalf("metadata = %#v, want hermes/test", metadata)
	}

	resources := instance.Resources(context.Background())
	if len(resources) != 6 {
		t.Fatalf("registered resource count = %d, want model assignment, profile, custom provider, MCP, messaging, and cron", len(resources))
	}
	wantResources := map[string]bool{
		"hermes_model_assignment":   false,
		"hermes_profile":            false,
		"hermes_mcp_server":         false,
		"hermes_messaging_platform": false,
		"hermes_cron_job":           false,
		"hermes_custom_provider":    false,
	}
	for _, factory := range resources {
		var resourceMetadata frameworkresource.MetadataResponse
		factory().Metadata(context.Background(), frameworkresource.MetadataRequest{}, &resourceMetadata)
		if _, ok := wantResources[resourceMetadata.TypeName]; !ok {
			t.Fatalf("unexpected resource type = %q", resourceMetadata.TypeName)
		}
		wantResources[resourceMetadata.TypeName] = true
	}
	for name, found := range wantResources {
		if !found {
			t.Fatalf("resource type %q was not registered", name)
		}
	}

	dataSources := instance.DataSources(context.Background())
	if len(dataSources) != 2 {
		t.Fatalf("registered data source count = %d, want hermes_status and hermes_model_options", len(dataSources))
	}
	wantDataSources := map[string]bool{
		"hermes_status":        false,
		"hermes_model_options": false,
	}
	for _, factory := range dataSources {
		var dataSourceMetadata frameworkdatasource.MetadataResponse
		factory().Metadata(context.Background(), frameworkdatasource.MetadataRequest{}, &dataSourceMetadata)
		if _, ok := wantDataSources[dataSourceMetadata.TypeName]; !ok {
			t.Fatalf("unexpected data source type = %q", dataSourceMetadata.TypeName)
		}
		wantDataSources[dataSourceMetadata.TypeName] = true
	}
	for name, found := range wantDataSources {
		if !found {
			t.Fatalf("data source type %q was not registered", name)
		}
	}
}

func TestMCPAndCronSchemaContracts(t *testing.T) {
	for _, test := range []struct {
		name     string
		instance frameworkresource.Resource
	}{
		{name: "MCP server", instance: mcp.NewResource()},
		{name: "messaging platform", instance: messaging.NewResource()},
		{name: "cron job", instance: cron.NewResource()},
		{name: "custom provider", instance: custom.NewResource()},
	} {
		t.Run(test.name, func(t *testing.T) {
			var response frameworkresource.SchemaResponse
			test.instance.Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("schema diagnostics: %v", response.Diagnostics)
			}
			assertNoDiagnostics(t, response.Schema.ValidateImplementation(context.Background()))
		})
	}
}

func TestProviderSchemaContract(t *testing.T) {
	providerSchemaValue := providerSchema()
	assertNoDiagnostics(t, providerSchemaValue.ValidateImplementation(context.Background()))

	password, ok := providerSchemaValue.Attributes["password"].(frameworkproviderschema.StringAttribute)
	if !ok {
		t.Fatal("password is not a string attribute")
	}
	if !password.Optional || !password.Sensitive {
		t.Fatalf("password schema = %#v, want optional and sensitive", password)
	}
	if _, ok := providerSchemaValue.Attributes["endpoint"]; !ok {
		t.Fatal("provider schema is missing endpoint")
	}
}

func TestModelAssignmentSchemaContract(t *testing.T) {
	instance := model.NewAssignmentResource()
	var response frameworkresource.SchemaResponse
	instance.Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("resource schema diagnostics: %v", response.Diagnostics)
	}
	assertNoDiagnostics(t, response.Schema.ValidateImplementation(context.Background()))

	attributes := response.Schema.Attributes
	for _, name := range []string{"id", "scope", "task", "model_provider", "model", "base_url", "api_key", "confirm_expensive_model", "profile"} {
		if _, ok := attributes[name]; !ok {
			t.Fatalf("resource schema is missing %q", name)
		}
	}

	modelProvider, ok := attributes["model_provider"].(frameworkresourceschema.StringAttribute)
	if !ok || !modelProvider.Required {
		t.Fatalf("model_provider schema = %#v, want required string", attributes["model_provider"])
	}
	apiKey, ok := attributes["api_key"].(frameworkresourceschema.StringAttribute)
	if !ok || !apiKey.Sensitive {
		t.Fatalf("api_key schema = %#v, want sensitive string", attributes["api_key"])
	}
}

func assertNoDiagnostics(t *testing.T, diagnostics interface{ HasError() bool }) {
	t.Helper()
	if diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", diagnostics)
	}
}
