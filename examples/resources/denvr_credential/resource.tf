# Une clé d'accès restreinte au seul espace « archives ».
# Attention : le secret est stocké dans l'état Terraform — protégez le backend.
resource "denvr_credential" "ci" {
  nom     = "cle-integration-continue"
  type    = "restreinte"
  buckets = [denvr_bucket.archives.id]
}
