package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/stevenhansel/terraform-provider-hermes/internal/provider"
)

var version = "dev"

func main() {
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/stevenhansel/hermes",
	}); err != nil {
		log.Fatal(err)
	}
}
