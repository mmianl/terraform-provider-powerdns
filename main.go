package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/terraform-providers/terraform-provider-powerdns/powerdns"
)

// version is set by the release build.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers")
	flag.Parse()

	err := providerserver.Serve(context.Background(), powerdns.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/mmianl/powerdns",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
