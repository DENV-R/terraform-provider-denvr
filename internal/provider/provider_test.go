package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories fournit le provider aux tests d'acceptance (protocole 6).
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"denvr": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifie les prerequis avant tout test d'acceptance : ces tests
// creent de VRAIES ressources sur un compte DENV-R de test (jeton lecture-ecriture).
func testAccPreCheck(t *testing.T) {
	if os.Getenv("DENVR_API_URL") == "" {
		t.Fatal("DENVR_API_URL doit etre defini pour les tests d'acceptance.")
	}
	if os.Getenv("DENVR_TOKEN") == "" {
		t.Fatal("DENVR_TOKEN (jeton lecture-ecriture) doit etre defini pour les tests d'acceptance.")
	}
}
