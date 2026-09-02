// SPDX-License-Identifier: AGPL-3.0-or-later

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name penguin

// Command terraform-provider-penguin is the official OpenTofu/Terraform provider
// for the Penguin game-server control panel — a soft-fork of Pelican. It manages
// the panel through its admin Application API.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/zephyrex/terraform-provider-penguin/internal/provider"
)

// version is overwritten at release build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/zephyrex/penguin",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
