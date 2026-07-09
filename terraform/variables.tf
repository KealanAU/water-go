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

# --- Networking --------------------------------------------------------------

variable "vpc_cidr" {
  description = "CIDR block for the dedicated VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of Availability Zones to spread subnets across"
  type        = number
  default     = 2
}

# --- Database ----------------------------------------------------------------

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

variable "db_engine_version" {
  description = "Postgres major version"
  type        = string
  default     = "16"
}

# --- Secrets -----------------------------------------------------------------

variable "nve_api_key" {
  description = "NVE HydAPI key, stored in AWS Secrets Manager"
  type        = string
  sensitive   = true
  default     = ""
}

variable "api_keys" {
  description = "Comma-separated API keys for the public data endpoints. Empty disables API auth."
  type        = string
  sensitive   = true
  default     = ""
}

variable "alert_webhook_url" {
  description = "Optional webhook URL for anomaly alerts emitted by the ingester"
  type        = string
  sensitive   = true
  default     = ""
}

# --- Compute -----------------------------------------------------------------

variable "container_image" {
  description = "Container image (ECR URI) for the ingester/api services. Defaults to the created repo at :latest."
  type        = string
  default     = ""
}

variable "task_cpu" {
  description = "Fargate task CPU units (1024 = 1 vCPU)"
  type        = number
  default     = 256
}

variable "task_memory" {
  description = "Fargate task memory in MiB"
  type        = number
  default     = 512
}

variable "api_desired_count" {
  description = "Number of api tasks to run behind the ALB"
  type        = number
  default     = 1
}

variable "ingester_desired_count" {
  description = "Number of ingester tasks to run"
  type        = number
  default     = 1
}

variable "api_port" {
  description = "Port the api container listens on"
  type        = number
  default     = 8080
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention for the service log groups"
  type        = number
  default     = 14
}

variable "api_rate_limit" {
  description = "Per-client API rate limit in requests per second. Set to 0 to disable."
  type        = number
  default     = 10
}

variable "api_rate_burst" {
  description = "Per-client API rate limit burst size"
  type        = number
  default     = 20
}

# --- Application tuning (optional container env) ------------------------------
#
# These map to the optional env vars the binaries understand. Left empty means
# "use the binary's own default" — empty values are filtered out before being
# passed to the container.

variable "poll_interval" {
  description = "Ingester poll interval (e.g. 15m)"
  type        = string
  default     = ""
}

variable "station_ids" {
  description = "Comma-separated NVE station IDs to ingest"
  type        = string
  default     = ""
}

variable "parameters" {
  description = "Comma-separated NVE parameters to fetch"
  type        = string
  default     = ""
}

variable "resolution_time" {
  description = "NVE resolution time value"
  type        = string
  default     = ""
}

variable "lookback" {
  description = "Ingester lookback window (e.g. 72h)"
  type        = string
  default     = ""
}

# --- Alerting ----------------------------------------------------------------

variable "alert_email" {
  description = "Optional email address subscribed to CloudWatch alarm notifications"
  type        = string
  default     = ""
}

variable "api_5xx_alarm_threshold" {
  description = "Target 5xx responses in a 5-minute window before alerting"
  type        = number
  default     = 5
}

variable "api_latency_alarm_threshold_seconds" {
  description = "Average ALB target response time threshold before alerting"
  type        = number
  default     = 2
}

variable "ecs_cpu_alarm_threshold" {
  description = "Average ECS service CPU utilization percentage before alerting"
  type        = number
  default     = 80
}

variable "rds_cpu_alarm_threshold" {
  description = "Average RDS CPU utilization percentage before alerting"
  type        = number
  default     = 80
}

variable "rds_free_storage_alarm_threshold_bytes" {
  description = "RDS free storage bytes below which to alert"
  type        = number
  default     = 2147483648
}
