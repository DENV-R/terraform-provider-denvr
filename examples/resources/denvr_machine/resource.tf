# Une machine virtuelle nue dans le réseau privé de l'espace : Debian 12, 2 vCPU, 2 Go, 20 Go,
# votre clé SSH. Facturée au prix de Paramètres (3 €/vCPU, 1 €/Go, 0,05 €/Go) à l'heure, sur la facture du mois suivant.
resource "denvr_machine" "bastion" {
  espace_id  = denvr_espace.prod.id
  modele     = "debian12" # debian12, ubuntu2404, alpine324
  hostname   = "bastion"
  coeurs     = 2
  memoire_mo = 2048
  disque_go  = 20
  cles_ssh   = file("~/.ssh/id_ed25519.pub")
}

output "bastion_ip_privee" {
  value = denvr_machine.bastion.ip_privee
}
