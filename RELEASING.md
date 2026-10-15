# Publier une version

Procédure de l'équipe DENV-R. Une version publiée ne se réécrit pas : un correctif = un nouveau tag.

1. Mettre la version dans `Makefile` (`VERSION`) et dans `CHANGELOG.md`.
2. `make check` puis `make docs` si un schéma ou un exemple a changé.
3. Pousser `main`, puis le tag :

   ```bash
   git tag -a v0.2.3 -m "v0.2.3"
   git push origin v0.2.3
   ```

4. Le workflow `.github/workflows/release.yml` lance GoReleaser : compilation pour chaque système,
   `SHA256SUMS` signé avec la clé GPG DENV-R (secrets `GPG_PRIVATE_KEY` et `PASSPHRASE` du dépôt),
   manifest du registre, release GitHub.
5. Le registre HashiCorp ingère la release seul (namespace `denv-r`, clé GPG déclarée dans
   HCP Terraform). Le registre DENV-R (`console.denv-r.com/denvr/denvr`) la sert après son
   rafraîchissement.
6. Vérifier : la page de la version apparaît sur les deux registres et `terraform init` dans
   `examples/` télécharge la nouvelle version.
