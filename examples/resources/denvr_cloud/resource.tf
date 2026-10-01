# Un stockage S3 externe connecté (ici un MinIO on-premise à certificat auto-signé,
# monté en lecture-écriture).
resource "denvr_cloud" "minio_labo" {
  provider_type = "minio"
  endpoint      = "https://minio.interne.example:9000"
  region        = "us-east-1"
  access_key    = var.minio_key
  secret_key    = var.minio_secret
  verify_tls    = false
  lecture_seule = false
}

variable "minio_key" { type = string }
variable "minio_secret" {
  type      = string
  sensitive = true
}
