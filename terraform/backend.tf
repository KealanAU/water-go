# Remote state backend.
#
# State is kept local by default so the project can be applied from a clean
# checkout without any pre-existing infrastructure. For team use (or anything
# long-lived), enable the S3 backend below. It is intentionally commented out
# because the bucket and lock table must exist *before* they can hold state —
# create them once, then uncomment and run `terraform init -migrate-state`.
#
# One-time bootstrap (adjust names/region):
#
#   aws s3api create-bucket \
#     --bucket water-go-tfstate \
#     --region eu-north-1 \
#     --create-bucket-configuration LocationConstraint=eu-north-1
#   aws s3api put-bucket-versioning \
#     --bucket water-go-tfstate \
#     --versioning-configuration Status=Enabled
#   aws dynamodb create-table \
#     --table-name water-go-tflock \
#     --attribute-definitions AttributeName=LockID,AttributeType=S \
#     --key-schema AttributeName=LockID,KeyType=HASH \
#     --billing-mode PAY_PER_REQUEST \
#     --region eu-north-1
#
# terraform {
#   backend "s3" {
#     bucket         = "water-go-tfstate"
#     key            = "water-go/terraform.tfstate"
#     region         = "eu-north-1"
#     dynamodb_table = "water-go-tflock" # state locking
#     encrypt        = true
#   }
# }
