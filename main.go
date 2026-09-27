// Copyright jambazid 2026
// SPDX-License-Identifier: MPL-2.0

// Package main is the entrypoint for the terraform-provider-fabricext binary.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/jambazid/terraform-provider-fabricext/internal/provider"
)

//go:generate terraform fmt -recursive ./examples/ ./modules/
//go:generate tfplugindocs generate --provider-name fabricext --rendered-provider-name "Fabric Extensions (fabricext)"

var (
	version = "dev"
	commit  = ""
)

var _ = commit

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/jambazid/fabricext",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
