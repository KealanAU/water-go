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
  count         = nonsensitive(var.nve_api_key) == "" ? 0 : 1
  secret_id     = aws_secretsmanager_secret.nve_api_key.id
  secret_string = var.nve_api_key
}

resource "aws_secretsmanager_secret" "api_keys" {
  name = "${var.project_name}-${var.environment}-api-keys"
}

resource "aws_secretsmanager_secret_version" "api_keys" {
  count         = nonsensitive(var.api_keys) == "" ? 0 : 1
  secret_id     = aws_secretsmanager_secret.api_keys.id
  secret_string = var.api_keys
}

resource "aws_secretsmanager_secret" "alert_webhook_url" {
  name = "${var.project_name}-${var.environment}-alert-webhook-url"
}

resource "aws_secretsmanager_secret_version" "alert_webhook_url" {
  count         = nonsensitive(var.alert_webhook_url) == "" ? 0 : 1
  secret_id     = aws_secretsmanager_secret.alert_webhook_url.id
  secret_string = var.alert_webhook_url
}

# aws_db_instance.endpoint is "host:port", which slots straight into the URL.
resource "aws_secretsmanager_secret" "database_url" {
  name = "${var.project_name}-${var.environment}-database-url"
}

resource "aws_secretsmanager_secret_version" "database_url" {
  secret_id     = aws_secretsmanager_secret.database_url.id
  secret_string = "postgres://${var.db_username}:${random_password.db.result}@${aws_db_instance.postgres.endpoint}/${var.db_name}?sslmode=require"
}
