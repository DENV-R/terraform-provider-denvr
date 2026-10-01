// terraform-provider-denvr : provider Terraform de DENV-R.
//
// Il traduit des ressources déclaratives (HCL) en appels sur l'API REST DENV-R —
// la même API que la console et la CLI. « Terraform du stockage » : on décrit
// l'état voulu, Terraform réconcilie.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/denv-r/terraform-provider-denvr/internal/provider"
)

// Renseigné par le linker au build (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Lancer le provider en mode debug (attache un débogueur).")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		// Adresse de publication au registre Terraform.
		Address: "registry.terraform.io/denv-r/denvr",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
