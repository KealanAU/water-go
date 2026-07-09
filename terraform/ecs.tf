locals {
  container_image = var.container_image != "" ? var.container_image : "${aws_ecr_repository.app.repository_url}:latest"

  # Optional tuning env vars — only forward the ones actually set, so empty
  # strings don't override the binaries' built-in defaults.
  ingester_optional_env_raw = {
    POLL_INTERVAL   = var.poll_interval
    STATION_IDS     = var.station_ids
    PARAMETERS      = var.parameters
    RESOLUTION_TIME = var.resolution_time
    LOOKBACK        = var.lookback
  }
  ingester_optional_env = [
    for k, v in local.ingester_optional_env_raw : { name = k, value = v } if v != ""
  ]

  api_env = [
    { name = "API_ADDR", value = ":${var.api_port}" },
    { name = "API_RATE_LIMIT", value = tostring(var.api_rate_limit) },
    { name = "API_RATE_BURST", value = tostring(var.api_rate_burst) },
  ]

  # Injected via valueFrom so secret values never appear in the task definition.
  db_secret = {
    name      = "DATABASE_URL"
    valueFrom = aws_secretsmanager_secret.database_url.arn
  }
  nve_secret = {
    name      = "NVE_API_KEY"
    valueFrom = aws_secretsmanager_secret.nve_api_key.arn
  }
  api_keys_secret = {
    name      = "API_KEYS"
    valueFrom = aws_secretsmanager_secret.api_keys.arn
  }
  alert_webhook_secret = {
    name      = "ALERT_WEBHOOK_URL"
    valueFrom = aws_secretsmanager_secret.alert_webhook_url.arn
  }
  ingester_secrets = concat(
    [local.db_secret],
    nonsensitive(var.nve_api_key) == "" ? [] : [local.nve_secret],
    nonsensitive(var.alert_webhook_url) == "" ? [] : [local.alert_webhook_secret],
  )
  api_secrets = concat(
    [local.db_secret],
    nonsensitive(var.api_keys) == "" ? [] : [local.api_keys_secret],
  )
}

resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-${var.environment}"
}

# Shared egress-only SG for both services; RDS references this to allow 5432.
resource "aws_security_group" "ecs_tasks" {
  name        = "${var.project_name}-${var.environment}-tasks"
  description = "ECS task ENIs"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "api from ALB"
    from_port       = var.api_port
    to_port         = var.api_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.project_name}-${var.environment}-tasks" }
}

# --- ingester ----------------------------------------------------------------

resource "aws_cloudwatch_log_group" "ingester" {
  name              = "/ecs/${var.project_name}-${var.environment}/ingester"
  retention_in_days = var.log_retention_days
}

resource "aws_ecs_task_definition" "ingester" {
  family                   = "${var.project_name}-${var.environment}-ingester"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.task_cpu
  memory                   = var.task_memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([{
    name        = "ingester"
    image       = local.container_image
    essential   = true
    command     = ["/usr/local/bin/ingester"]
    secrets     = local.ingester_secrets
    environment = local.ingester_optional_env
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.ingester.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ingester"
      }
    }
  }])

  depends_on = [
    aws_secretsmanager_secret_version.database_url,
    aws_secretsmanager_secret_version.nve_api_key,
    aws_secretsmanager_secret_version.alert_webhook_url,
  ]
}

resource "aws_ecs_service" "ingester" {
  name            = "ingester"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.ingester.arn
  desired_count   = var.ingester_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }
}

# --- api ---------------------------------------------------------------------

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${var.project_name}-${var.environment}/api"
  retention_in_days = var.log_retention_days
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.project_name}-${var.environment}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.task_cpu
  memory                   = var.task_memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  container_definitions = jsonencode([{
    name        = "api"
    image       = local.container_image
    essential   = true
    command     = ["/usr/local/bin/api"]
    secrets     = local.api_secrets
    environment = local.api_env
    portMappings = [{
      containerPort = var.api_port
      protocol      = "tcp"
    }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.api.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "api"
      }
    }
  }])

  depends_on = [
    aws_secretsmanager_secret_version.database_url,
    aws_secretsmanager_secret_version.api_keys,
  ]
}

resource "aws_ecs_service" "api" {
  name            = "api"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = var.api_port
  }

  # Ensure the listener/target group exist before the service tries to register.
  depends_on = [aws_lb_listener.http]
}
