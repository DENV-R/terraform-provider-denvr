# Une politique du control plane : rétention minimale de 90 jours.
resource "denvr_policy" "retention" {
  type   = "retention"
  actif  = true
  params = jsonencode({ min_jours = 90 })
}
