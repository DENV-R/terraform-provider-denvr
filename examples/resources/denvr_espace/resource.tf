# Un espace sur le socle mutualisé DENV-R : réseau privé + cluster Kubernetes de 3 nœuds (taille S),
# prêt en une dizaine de minutes. `terraform destroy` demande la fermeture à l'équipe (rien n'est
# détruit par le provider lui-même).
resource "denvr_espace" "prod" {
  code   = "acme"
  taille = "S" # S, M, L (cluster) ou VPC (réseau privé seul)
}

output "espace_adresse_publique" {
  value = denvr_espace.prod.adresse_publique
}

output "espace_connexion_kubernetes" {
  description = "Identifiant de la connexion Kubernetes posée par la console : déployez vos montages dessus."
  value       = denvr_espace.prod.connexion_id
}
