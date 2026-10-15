# Historique des versions

Les versions suivent `MAJEUR.MINEUR.CORRECTIF`. Une version publiée ne se réécrit jamais :
un correctif donne une nouvelle version.

## 0.2.2 — 14 octobre 2026

- Facturation à l'heure dite dans les schémas et la documentation : un espace et une machine
  sont facturés pour les heures existées, heure commencée due, sur la facture du mois suivant.

## 0.2.1 — 14 octobre 2026

- Textes de facturation alignés sur ce que la console facture réellement.

## 0.2.0 — 14 octobre 2026

- Le socle de calcul mutualisé DENV-R en trois ressources : `denvr_espace` (réseau privé et
  cluster Kubernetes de trois nœuds, ou réseau seul), `denvr_machine` (machine virtuelle nue
  Debian, Ubuntu ou Alpine) et `denvr_redirections` (liste complète des ports publiés).
- `terraform destroy` d'un espace demande la fermeture à l'équipe ; le provider ne détruit rien
  sur le socle.
- Parité avec la console et l'API : mêmes règles, mêmes refus, mêmes raisons.

## 0.1.0 — 6 octobre 2026

- Première publication sur le registre HashiCorp et sur le registre DENV-R.
- Quatre ressources de stockage : `denvr_bucket`, `denvr_credential`, `denvr_cloud`, `denvr_policy`.
- Mode « immuabilité gérée par l'application » sur `denvr_bucket` (Object Lock sans rétention par
  défaut, pour Veeam et consorts).
- Releases signées par GoReleaser à chaque tag `vX.Y.Z`.
