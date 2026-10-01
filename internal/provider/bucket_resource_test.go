package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccBucketResource : cycle de vie (create + read) d'un espace de stockage.
// Test d'ACCEPTANCE : ne s'execute que si TF_ACC est defini et cree une vraie
// ressource sur le compte de test cible par DENVR_API_URL / DENVR_TOKEN.
func TestAccBucketResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "denvr_bucket" "test" {
  nom       = "acc-test-denvr"
  offre     = "standard"
  taille_to = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("denvr_bucket.test", "nom", "acc-test-denvr"),
					resource.TestCheckResourceAttrSet("denvr_bucket.test", "id"),
				),
			},
		},
	})
}
