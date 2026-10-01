# terraform-provider-denvr

Provider Terraform de DENV-R : décrivez votre stockage souverain en **Infrastructure
as Code**. Il traduit des ressources HCL en appels sur l'API REST DENV-R — la même
API que la console et la CLI. Vous déclarez l'état voulu, Terraform réconcilie.

> **État : v0.1, prêt à publier.** Quatre ressources — `denvr_bucket`,
> `denvr_credential`, `denvr_cloud`, `denvr_policy` — en CRUD complet, cohérentes avec
> l'API en production. `go build`, `go vet`, `gofmt`, `go test` : **OK**
> (terraform-plugin-framework v1.19). Documentation Registry générée (`docs/`),
> chaîne de release en place (`.goreleaser.yml` + workflow GitHub), squelette de tests
> d'acceptance (`make testacc`, requiert un compte de test). Reste l'acte de
> publication (voir « Publication » plus bas). Toujours lancer un `terraform plan` sur
> un compte de test avant la production.

## Ressources

| Ressource | Rôle | Endpoints API |
|---|---|---|
| `denvr_bucket` | Espace de stockage S3 | `POST/GET/PATCH/DELETE /buckets` |
| `denvr_credential` | Clé d'accès (ROOT ou restreinte) | `POST/GET/PATCH/DELETE /credentials` |
| `denvr_cloud` | Connexion à un S3 externe (provider ou on-premise) | `POST/GET/PATCH/DELETE /clouds` |
| `denvr_policy` | Politique du control plane (rétention/souveraineté/immuabilité) | `PUT/GET /policies` |

### `denvr_bucket`

```hcl
resource "denvr_bucket" "archives" {
  nom       = "archives-2026"   # non modifiable après création (ForceNew)
  offre     = "standard"        # modifiable en place
  taille_to = 20                # modifiable en place
  versioning = true             # modifiable en place
  tags      = "SAUVEGARDE"      # modifiable en place

  # Verrouillage WORM (non modifiable après création) :
  # object_locking_days = 3650
  # object_lock_mode    = "COMPLIANCE"
}
```

| Attribut | Écriture | Remarque |
|---|---|---|
| `nom` | création seule | ForceNew |
| `offre`, `taille_to`, `versioning`, `tags` | création + modification | `PATCH` en place |
| `multisite`, `object_locking_days`, `object_lock_mode` | création seule | ForceNew |
| `id` | calculé | identifiant interne |

Mapping API : `POST /api/v1/buckets/` (create) · `GET /api/v1/buckets/{id}/` (read) ·
`PATCH /api/v1/buckets/{id}/` (update) · `DELETE /api/v1/buckets/{id}/` (delete).
Import : `terraform import denvr_bucket.archives <id>`.

### `denvr_credential`

```hcl
resource "denvr_credential" "ci" {
  nom     = "cle-integration-continue"
  type    = "restreinte"                    # ou "root" (accès total). ForceNew.
  buckets = [denvr_bucket.archives.id]      # espaces autorisés (type restreinte)
}
```

Attributs calculés : `access_key`, `secret_key` (**sensible**), `isolee`.
Import : `terraform import denvr_credential.ci <id>`.

### `denvr_cloud`

```hcl
resource "denvr_cloud" "ovh_prod" {
  provider_type = "ovh"                      # ForceNew ; « provider » est réservé par Terraform
  endpoint      = "s3.gra.io.cloud.ovh.net"
  region        = "gra"
  access_key    = var.ovh_key
  secret_key    = var.ovh_secret             # sensible
  verify_tls    = true                       # false pour un on-premise auto-signé
  lecture_seule = false                      # false = autorise l'écriture/migration
}
```

Import : `terraform import denvr_cloud.ovh_prod <id>`.

### `denvr_policy`

```hcl
resource "denvr_policy" "retention" {
  type   = "retention"                       # ou "souverainete" / "immutabilite". ForceNew.
  actif  = true
  params = jsonencode({ min_jours = 90 })    # souverainete : { zones = ["FR","EU"] }
}
```

Une seule politique par `type` (l'API fait un upsert). « Supprimer » la ressource
la **désactive** (l'API ne supprime pas un type). Import : `terraform import denvr_policy.retention retention`
(l'identifiant est le type).

## ⚠️ Secrets et état Terraform

`denvr_credential.secret_key` et `denvr_cloud.secret_key` sont stockés **en clair dans
le fichier d'état** (`terraform.tfstate`) — comportement normal de Terraform. Utilisez
un backend distant chiffré et verrouillé (S3+DynamoDB, Terraform Cloud…) ; ne commitez
jamais l'état. Ces attributs sont marqués `sensitive` (masqués des logs et du plan).

## Configuration

```hcl
provider "denvr" {
  url   = "https://console.denv-r.com"  # ou variable d'env DENVR_API_URL
  token = "dvr_xxx"                     # ou variable d'env DENVR_TOKEN (recommandé)
}
```

Le jeton doit être de portée **`read-write`** (créé depuis la console, Profil → API).

## Développement

```bash
make tidy     # récupère/aligne les dépendances
make build    # compile le binaire (avec la version)
make check    # gofmt + go vet
make testacc  # tests d'acceptance (requiert DENVR_API_URL + DENVR_TOKEN read-write + TF_ACC)
make docs     # régénère docs/ (requiert tfplugindocs)
make install  # installe en local pour un essai (adapter OS_ARCH)
```

Essai local sans registre : `make install`, puis dans un dossier de test
`terraform init` trouvera le provider dans `~/.terraform.d/plugins/...`.

## Publication au Terraform Registry

Le Registry public **ne publie que depuis GitHub** : ce dossier doit vivre dans un dépôt
GitHub dédié `terraform-provider-denvr` (miroir du code, aujourd'hui sur GitLab).

1. **Clé GPG de signature** (une fois) : générer une clé, publier la clé publique, et la
   déclarer dans le compte du Registry (Settings → Signing keys).
2. **Secrets GitHub** du dépôt : `GPG_PRIVATE_KEY` (clé privée ASCII-armored) et
   `PASSPHRASE`.
3. **Revendiquer le namespace** sur registry.terraform.io (« Publish → Provider »,
   organisation `denv-r`).
4. **Publier une version** : `git tag v0.1.0 && git push origin v0.1.0`. Le workflow
   `.github/workflows/release.yml` lance GoReleaser (build multi-plateforme +
   `SHA256SUMS` signés + manifest) et crée la release GitHub ; le Registry l'ingère.
5. **Vérifier** : la page `registry.terraform.io/providers/denv-r/denvr` apparaît, et un
   `terraform init` depuis `examples/` télécharge le provider.

Correctif : bump de version dans le tag (`v0.1.1`) puis re-push — le Registry est
append-only, une version publiée ne se réécrit pas.

## Feuille de route

- Data sources (lecture seule) : lister espaces/clés/clouds, exposer le plan de dérive.
- Import (`terraform import`) des ressources existantes.
