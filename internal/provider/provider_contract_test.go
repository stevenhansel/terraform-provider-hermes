package provider

import (
	"context"
	"testing"

	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	frameworkproviderschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProviderMetadataAndResources(t *testing.T) {
	instance := &Provider{version: "test"}

	var metadata frameworkprovider.MetadataResponse
	instance.Metadata(context.Background(), frameworkprovider.MetadataRequest{}, &metadata)
	if metadata.TypeName != "hermes" || metadata.Version != "test" {
		t.Fatalf("metadata = %#v, want hermes/test", metadata)
	}

	resources := instance.Resources(context.Background())
	if len(resources) != 0 {
		t.Fatalf("registered resource count = %d, want none", len(resources))
	}
	wantResources := map[string]bool{}
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
	if len(dataSources) != 0 {
		t.Fatalf("registered data source count = %d, want none", len(dataSources))
	}
	wantDataSources := map[string]bool{}
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

func assertNoDiagnostics(t *testing.T, diagnostics interface{ HasError() bool }) {
	t.Helper()
	if diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", diagnostics)
	}
}
