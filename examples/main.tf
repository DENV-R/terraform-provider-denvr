terraform {
  required_providers {
    denvr = {
      source = "denv-r/denvr"
    }
  }
}

# URL et jeton peuvent aussi venir des variables d'environnement
# DENVR_API_URL et DENVR_TOKEN (recommandé : ne pas écrire le jeton en clair).
provider "denvr" {
  url = "https://console.denv-r.com"
  # token = "dvr_xxx"   # sinon export DENVR_TOKEN=...
}

resource "denvr_bucket" "archives" {
  nom       = "archives-2026"
  offre     = "standard"
  taille_to = 20
  tags      = "SAUVEGARDE"
}

resource "denvr_bucket" "coffre" {
  nom                 = "coffre-legal"
  offre               = "standard"
  taille_to           = 5
  versioning          = true
  object_locking_days = 3650
  object_lock_mode    = "COMPLIANCE" # irréversible : le changer forcera un remplacement
}

# Un espace pour un logiciel de sauvegarde immuable (Veeam, Commvault, Rubrik…) :
# Object Lock activé, SANS rétention par défaut. Exclusif avec object_locking_days.
resource "denvr_bucket" "repo_veeam" {
  nom              = "repo-veeam"
  offre            = "standard"
  taille_to        = 50
  immutabilite_app = true
}

# Une clé d'accès restreinte au seul espace « archives » (le secret finit dans l'état
# Terraform : protégez le backend).
resource "denvr_credential" "ci" {
  nom     = "cle-integration-continue"
  type    = "restreinte"
  buckets = [denvr_bucket.archives.id]
}

# Un cloud externe connecté (ici on-premise MinIO à certificat auto-signé, en écriture).
resource "denvr_cloud" "minio_labo" {
  provider_type = "minio"
  endpoint      = "https://minio.interne.example:9000"
  region        = "us-east-1"
  access_key    = var.minio_key
  secret_key    = var.minio_secret
  verify_tls    = false
  lecture_seule = false
}

# Une politique de rétention déclarée (control plane).
resource "denvr_policy" "retention" {
  type   = "retention"
  actif  = true
  params = jsonencode({ min_jours = 90 })
}

variable "minio_key" { type = string }
variable "minio_secret" {
  type      = string
  sensitive = true
}

output "archives_id" {
  value = denvr_bucket.archives.id
}

output "ci_access_key" {
  value = denvr_credential.ci.access_key
}
