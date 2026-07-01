# Secrets Manager holds everything the containers shouldn't see in plaintext:
# the generated DB password, the NVE API key, and the fully composed
# DATABASE_URL. The last one is injected into ECS via `secrets` (valueFrom) so
# the connection string never appears in the task definition or console.

resource "aws_secretsmanager_secret" "db_password" {
  name = "${var.project_name}-${var.environment}-db-password"
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db.result
}

resource "aws_secretsmanager_secret" "nve_api_key" {
  name = "${var.project_name}-${var.environment}-nve-api-key"
}

resource "aws_secretsmanager_secret_version" "nve_api_key" {
  count         = var.nve_api_key == "" ? 0 : 1
  secret_id     = aws_secretsmanager_secret.nve_api_key.id
  secret_string = var.nve_api_key
}

# aws_db_instance.endpoint is "host:port", which slots straight into the URL.
resource "aws_secretsmanager_secret" "database_url" {
  name = "${var.project_name}-${var.environment}-database-url"
}

resource "aws_secretsmanager_secret_version" "database_url" {
  secret_id     = aws_secretsmanager_secret.database_url.id
  secret_string = "postgres://${var.db_username}:${random_password.db.result}@${aws_db_instance.postgres.endpoint}/${var.db_name}?sslmode=require"
}
