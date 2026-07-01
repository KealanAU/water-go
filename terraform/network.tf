# Dedicated VPC with public + private subnets across `az_count` AZs.
#
# - Public subnets host the internet-facing ALB and the NAT gateway.
# - Private subnets host the ECS tasks and RDS, so neither is reachable from
#   the internet directly; outbound access (ECR pulls, NVE API calls) goes
#   through a single NAT gateway to keep cost down.
locals {
  azs = slice(data.aws_availability_zones.available.names, 0, var.az_count)

  # Carve /24s out of the VPC CIDR: public subnets first, then private.
  public_subnets  = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 8, i)]
  private_subnets = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 8, i + var.az_count)]
}

data "aws_availability_zones" "available" {
  state = "available"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.project_name}-${var.environment}"
  cidr = var.vpc_cidr

  azs             = local.azs
  public_subnets  = local.public_subnets
  private_subnets = local.private_subnets

  # Single NAT gateway shared across AZs — cheaper, adequate for a portfolio app.
  enable_nat_gateway     = true
  single_nat_gateway     = true
  one_nat_gateway_per_az = false

  enable_dns_hostnames = true
  enable_dns_support   = true
}
