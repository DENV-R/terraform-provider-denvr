# Provider Terraform DENV-R

[![Version](https://img.shields.io/github/v/release/DENV-R/terraform-provider-denvr?label=version&color=0e7490)](https://github.com/DENV-R/terraform-provider-denvr/releases)
[![Registre HashiCorp](https://img.shields.io/badge/registry.terraform.io-denv--r%2Fdenvr-5c4ee5)](https://registry.terraform.io/providers/DENV-R/denvr)
[![Registre DENV-R](https://img.shields.io/badge/console.denv--r.com-denvr%2Fdenvr-0e7490)](https://console.denv-r.com/terraform/v1/providers/denvr/denvr/versions)

Décrivez votre système d'information chez DENV-R en code : espaces de stockage, clés d'accès,
clouds connectés, règles, et le socle de calcul (réseau privé, cluster Kubernetes, machines,
ports publiés). Le provider parle à la même API REST que la console et la CLI `denv-r` :
ce que vous déclarez ici est exactement ce que la console affiche.

> *English — Terraform provider for DENV-R, a sovereign cloud hosted in France. Resources for
> S3 storage, access keys, connected clouds, policies and the shared compute base (private
> network, Kubernetes cluster, bare VMs, published ports). Documentation is in French.*

## Installation

Deux adresses, le même binaire signé :

```hcl
terraform {
  required_providers {
    denvr = {
      source  = "denv-r/denvr"                    # registre HashiCorp
      # source = "console.denv-r.com/denvr/denvr" # ou notre propre registre
      version = ">= 0.2.2"
    }
  }
}

provider "denvr" {
  url = "https://console.denv-r.com"
  # Le jeton vient de la variable d'environnement DENVR_TOKEN (Profil → API, portée lecture-écriture).
}
```

## Premier fichier

```hcl
resource "denvr_bucket" "archives" {
  nom        = "archives-2026"
  offre      = "standard"
  taille_to  = 20
  versioning = true
}

resource "denvr_credential" "sauvegarde" {
  nom     = "cle-sauvegarde"
  type    = "restreinte"
  buckets = [denvr_bucket.archives.id]
}

output "access_key" { value = denvr_credential.sauvegarde.access_key }
```

`terraform apply` crée l'espace et sa clé ; `terraform plan` vous dit ensuite ce qui a dérivé.

## Votre SI existant, déjà en code

Pas besoin de réécrire ce qui existe. La console génère le fichier Terraform de **tout ce que
votre compte possède**, avec les secrets en variables, et le script d'import correspondant :

- dans la console : *Vos règles → Emporter en Terraform* ;
- en ligne de commande : `denv-r export terraform --imports` ;
- par l'API : `GET /api/v1/architecture/terraform/`.

Le fichier produit passe `terraform validate` et `terraform fmt -check` tel quel.

## Ressources

| Ressource | Ce qu'elle décrit |
|---|---|
| `denvr_bucket` | Un espace de stockage S3 : offre, taille, versions, verrouillage WORM, mode « géré par l'application » |
| `denvr_credential` | Une clé d'accès, totale ou restreinte à des espaces |
| `denvr_cloud` | Un stockage S3 externe connecté (hébergeur ou sur site), pour migrer ou synchroniser |
| `denvr_policy` | Une règle du plan de contrôle : rétention, souveraineté, immuabilité |
| `denvr_espace` | Un espace sur le socle DENV-R : réseau privé et, selon la taille, un cluster Kubernetes de trois nœuds |
| `denvr_machine` | Une machine virtuelle nue (Debian, Ubuntu, Alpine) dans le réseau privé d'un espace |
| `denvr_redirections` | La liste complète des ports publiés depuis l'adresse publique de l'espace vers ses machines |

Chaque ressource est documentée dans [docs/resources/](docs/resources/), avec un exemple
complet dans [examples/resources/](examples/resources/). Un exemple de bout en bout :
[examples/main.tf](examples/main.tf).

### Le socle en trois ressources

```hcl
resource "denvr_espace" "prod" {
  code   = "acme"
  taille = "S" # S, M, L : cluster de 3 nœuds ; VPC : réseau privé seul
}

resource "denvr_machine" "bastion" {
  espace_id  = denvr_espace.prod.id
  modele     = "debian12"
  hostname   = "bastion"
  coeurs     = 2
  memoire_mo = 2048
  disque_go  = 20
  cles_ssh   = file("~/.ssh/id_ed25519.pub")
}

resource "denvr_redirections" "prod" {
  espace_id = denvr_espace.prod.id
  redirection {
    proto       = "tcp"
    public_port = 2222
    target_ip   = split("/", denvr_machine.bastion.ip_privee)[0]
    target_port = 22
  }
}
```

## Ce que le provider fait, et ne fait pas

- **Un espace du socle est demandé, puis suivi.** La ressource est appliquée quand l'espace est
  prêt, une dizaine de minutes pour un cluster. Il est facturé à l'heure, de la mise à disposition
  à la fermeture, sur la facture du mois suivant.
- **Rien n'est détruit sur le socle par le provider.** `terraform destroy` d'un espace ouvre une
  demande de fermeture à l'équipe DENV-R ; celui d'une machine retire la machine de l'état
  seulement. C'est voulu : la console ne détient aucun pouvoir de suppression sur le socle.
- **Les ports publiés forment une liste complète**, appliquée sur les deux passerelles ou refusée.
  Jamais le port public 22 ; 80, 443 et 6443 restent au cluster quand l'espace en a un.
- **Une politique n'est jamais supprimée**, seulement désactivée (une par type, l'API fait un upsert).
- **Les secrets vivent dans l'état Terraform**, en clair, comme pour tout provider :
  `denvr_credential.secret_key` et `denvr_cloud.secret_key` sont marqués sensibles, mais protégez
  votre backend (distant, chiffré, verrouillé) et ne versionnez jamais `terraform.tfstate`.

## Jetons et rôles

Le jeton se crée dans la console (*Profil → API*). Lire demande une portée lecture ; appliquer
demande la portée lecture-écriture. Engager un prix (un espace, une machine, une fermeture) exige
le rôle propriétaire ou administrateur du compte ; publier un port est aussi ouvert au rôle
développeur. Un refus revient toujours avec sa raison, en français.

## Développement

```bash
make build    # compile le binaire
make check    # gofmt + go vet
make testacc  # tests d'acceptance : créent de VRAIES ressources (DENVR_API_URL, DENVR_TOKEN, TF_ACC=1)
make docs     # régénère docs/ depuis les schémas et examples/ (tfplugindocs)
make install  # installe en local pour un essai sans registre
```

Construit avec [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework).
Les versions sont publiées par un tag `vX.Y.Z` : GoReleaser compile pour chaque système, signe les
sommes de contrôle avec la clé GPG DENV-R et crée la release, que les deux registres ingèrent.
Le détail est dans [RELEASING.md](RELEASING.md), l'historique dans [CHANGELOG.md](CHANGELOG.md).

## Support

- Console : [console.denv-r.com](https://console.denv-r.com), documentation de l'API dans *Documentation*.
- Ligne de commande : `pip install denv-r-cli`, commande `denv-r`.
- Une question, un bug : support@denv-r.com ou une *issue* ici.

## Licence

[Apache License 2.0](LICENSE). Le code est libre d'être lu, audité, modifié et redistribué ; la marque DENV-R
reste celle de DENV-R SAS.
