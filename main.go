// terraform-provider-nile is a Terraform provider for the Nile control plane
// API (https://www.thenile.dev).
package main

import (
	"context"
	"log"

	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/mal-2/terraform-provider-nile/internal/provider"
)

// Provider metadata. These values are baked into the binary at build time
// via ldflags when releasing (see Makefile).
var (
	version string = "dev"
)

func main() {
	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/mal-2/nile",
	}

	newProvider := func() tfprovider.Provider {
		return provider.New(version)
	}

	err := providerserver.Serve(context.Background(), newProvider, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
