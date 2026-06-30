# AWS skeleton for water-go.
#
# Provisions the pieces the application needs: a container registry, secrets,
# and a managed Postgres instance (TimescaleDB runs as an extension on RDS).
# The ECS/Fargate services that run the ingester and api are stubbed at the
# bottom as a TODO — wire them up once you publish an image to ECR.

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

resource "aws_ecr_repository" "app" {
  name                 = var.project_name
  image_tag_mutability = "MUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "random_password" "db" {
  length  = 24
  special = false
}

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

resource "aws_security_group" "db" {
  name        = "${var.project_name}-${var.environment}-db"
  description = "Postgres access for water-go"
  vpc_id      = data.aws_vpc.default.id

  # NOTE: tighten this to the app security group before any real deployment.
  ingress {
    description = "Postgres"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.default.cidr_block]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.project_name}-${var.environment}"
  subnet_ids = data.aws_subnets.default.ids
}

resource "aws_db_instance" "postgres" {
  identifier              = "${var.project_name}-${var.environment}"
  engine                  = "postgres"
  engine_version          = "16"
  instance_class          = var.db_instance_class
  allocated_storage       = var.db_allocated_storage
  db_name                 = var.db_name
  username                = var.db_username
  password                = random_password.db.result
  db_subnet_group_name    = aws_db_subnet_group.this.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  skip_final_snapshot     = true
  publicly_accessible     = false
  backup_retention_period = 7
  apply_immediately       = true
}

# --- TODO: compute -----------------------------------------------------------
#
# Run the ingester and api on ECS Fargate (or App Runner). Sketch:
#
# resource "aws_ecs_cluster" "this" { name = "${var.project_name}-${var.environment}" }
#
# resource "aws_ecs_task_definition" "ingester" {
#   family                   = "${var.project_name}-ingester"
#   requires_compatibilities = ["FARGATE"]
#   network_mode             = "awsvpc"
#   cpu                      = "256"
#   memory                   = "512"
#   container_definitions    = jsonencode([{
#     name  = "ingester"
#     image = var.container_image  # "${aws_ecr_repository.app.repository_url}:latest"
#     command = ["/usr/local/bin/ingester"]
#     secrets = [
#       { name = "NVE_API_KEY", valueFrom = aws_secretsmanager_secret.nve_api_key.arn },
#     ]
#     environment = [
#       { name = "DATABASE_URL", value = "postgres://${var.db_username}:...@${aws_db_instance.postgres.endpoint}/${var.db_name}" },
#     ]
#   }])
# }
#
# A second task definition / service runs ["/usr/local/bin/api"] behind an ALB.
