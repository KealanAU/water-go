output "ecr_repository_url" {
  description = "Push the application image here"
  value       = aws_ecr_repository.app.repository_url
}

output "db_endpoint" {
  description = "RDS Postgres endpoint"
  value       = aws_db_instance.postgres.endpoint
}

output "db_password_secret_arn" {
  description = "Secrets Manager ARN holding the generated DB password"
  value       = aws_secretsmanager_secret.db_password.arn
}

output "nve_api_key_secret_arn" {
  description = "Secrets Manager ARN holding the NVE HydAPI key"
  value       = aws_secretsmanager_secret.nve_api_key.arn
}
