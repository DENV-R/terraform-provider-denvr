# Un espace de stockage standard.
resource "denvr_bucket" "archives" {
  nom       = "archives-2026"
  offre     = "standard"
  taille_to = 20
  tags      = "SAUVEGARDE"
}

# Un coffre immuable (WORM) : rétention verrouillée 10 ans, mode CONFORMITÉ
# (irréversible : modifier le mode force un remplacement de la ressource).
resource "denvr_bucket" "coffre" {
  nom                 = "coffre-legal"
  offre               = "standard"
  taille_to           = 5
  versioning          = true
  object_locking_days = 3650
  object_lock_mode    = "COMPLIANCE"
}

# Un espace pour un logiciel de sauvegarde immuable (Veeam, Commvault, Rubrik…) :
# Object Lock activé, SANS rétention par défaut (le logiciel pose l'immuabilité
# objet par objet). Exclusif avec object_locking_days.
resource "denvr_bucket" "repo_veeam" {
  nom              = "repo-veeam"
  offre            = "standard"
  taille_to        = 50
  immutabilite_app = true
}
