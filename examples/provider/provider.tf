terraform {
  required_providers {
    denvr = {
      source = "denv-r/denvr"
    }
  }
}

# URL et jeton peuvent aussi venir des variables d'environnement DENVR_API_URL et
# DENVR_TOKEN (recommandé : ne pas écrire le jeton en clair dans le code).
provider "denvr" {
  url = "https://console.denv-r.com"
  # token = "dvr_xxx"
}
