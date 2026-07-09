# Two roles per ECS convention:
# - execution role: used by the ECS agent to pull the image, write logs, and
#   fetch the secrets that get injected into the container.
# - task role: assumed by the running container itself (empty for now; the app
#   talks to RDS/NVE over the network, not via AWS APIs).

data "aws_iam_policy_document" "ecs_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.project_name}-${var.environment}-exec"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

# Grants ECR pull + CloudWatch Logs.
resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Allow the execution role to read exactly the secrets we inject — no wildcard.
data "aws_iam_policy_document" "secrets_access" {
  statement {
    actions = ["secretsmanager:GetSecretValue"]
    resources = [
      aws_secretsmanager_secret.database_url.arn,
      aws_secretsmanager_secret.nve_api_key.arn,
      aws_secretsmanager_secret.api_keys.arn,
      aws_secretsmanager_secret.alert_webhook_url.arn,
    ]
  }
}

resource "aws_iam_role_policy" "execution_secrets" {
  name   = "${var.project_name}-${var.environment}-secrets"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.secrets_access.json
}

resource "aws_iam_role" "task" {
  name               = "${var.project_name}-${var.environment}-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}
