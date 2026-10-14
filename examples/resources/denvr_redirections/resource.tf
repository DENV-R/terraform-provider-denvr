# Les ports publiés depuis l'adresse publique de l'espace vers ses machines : la liste COMPLÈTE,
# appliquée à chaud sur les deux passerelles. Jamais le port public 22 ; 80, 443 et 6443 sont
# réservés au cluster quand l'espace en a un.
resource "denvr_redirections" "prod" {
  espace_id = denvr_espace.prod.id

  redirection {
    proto       = "tcp"
    public_port = 8443
    target_ip   = split("/", denvr_machine.bastion.ip_privee)[0]
    target_port = 443
  }

  redirection {
    proto       = "udp"
    public_port = 51820
    target_ip   = split("/", denvr_machine.bastion.ip_privee)[0]
    target_port = 51820
  }
}
