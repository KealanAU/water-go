# Container registry for the single image that ships both binaries
# (ingester + api are selected at runtime via the container command).
resource "aws_ecr_repository" "app" {
  name                 = var.project_name
  image_tag_mutability = "MUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }
}
