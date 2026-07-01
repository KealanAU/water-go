output "alb_dns_name" {
  description = "Public DNS name of the api load balancer"
  value       = aws_lb.api.dns_name
}

output "ecr_repository_url" {
  description = "Push the application image here"
  value       = aws_ecr_repository.app.repository_url
}

output "db_endpoint" {
  description = "RDS Postgres endpoint (host:port)"
  value       = aws_db_instance.postgres.endpoint
}

output "ecs_cluster_name" {
  description = "ECS cluster running the ingester and api services"
  value       = aws_ecs_cluster.this.name
}

output "db_password_secret_arn" {
  description = "Secrets Manager ARN holding the generated DB password"
  value       = aws_secretsmanager_secret.db_password.arn
}

output "database_url_secret_arn" {
  description = "Secrets Manager ARN holding the composed DATABASE_URL"
  value       = aws_secretsmanager_secret.database_url.arn
}

output "nve_api_key_secret_arn" {
  description = "Secrets Manager ARN holding the NVE HydAPI key"
  value       = aws_secretsmanager_secret.nve_api_key.arn
}
