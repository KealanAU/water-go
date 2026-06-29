variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "eu-north-1" # Stockholm — closest to Norwegian data source
}

variable "project_name" {
  description = "Project name, used for tagging and resource naming"
  type        = string
  default     = "water-go"
}

variable "environment" {
  description = "Deployment environment (e.g. dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "db_instance_class" {
  description = "RDS instance class for the TimescaleDB-capable Postgres instance"
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage in GB"
  type        = number
  default     = 20
}

variable "db_name" {
  description = "Initial database name"
  type        = string
  default     = "water"
}

variable "db_username" {
  description = "Master database username"
  type        = string
  default     = "water"
}

variable "nve_api_key" {
  description = "NVE HydAPI key, stored in AWS Secrets Manager"
  type        = string
  sensitive   = true
  default     = ""
}

variable "container_image" {
  description = "Container image (ECR URI) for the ingester/api services"
  type        = string
  default     = ""
}
