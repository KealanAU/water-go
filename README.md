# water-go

Ingestion and API pipeline for Norwegian hydrological time-series data from the
[NVE HydAPI](https://hydapi.nve.no).

```
Poll NVE  →  publish raw_nve_observation  →  normalise (units/tz/quality)
          →  store in TimescaleDB         →  expose via HTTP API
```

## Stack

| Concern            | Choice                                   |
| ------------------ | ---------------------------------------- |
| Ingestion / churn  | [Watermill](https://watermill.io) (in-process gochannel pub/sub) |
| Database           | TimescaleDB (Postgres + hypertables)     |
| DB driver          | [pgx/v5](https://github.com/jackc/pgx)   |
| Type-safe queries  | [sqlc](https://sqlc.dev)                 |
| HTTP API           | [chi](https://github.com/go-chi/chi)     |
| Containers         | Docker + docker-compose                  |
| Infrastructure     | Terraform (AWS skeleton)                 |

## Layout

```
cmd/ingester     poller → watermill → normalizer → store
cmd/api          chi HTTP API over stored data
internal/nve     NVE HydAPI client
internal/pipeline Watermill poller + normalizer
internal/store   pgx pool + migrations + sqlc queries
internal/db      sqlc-generated code (run `make sqlc`)
db/migrations    TimescaleDB schema
db/queries       sqlc query definitions
terraform/       AWS skeleton (ECR, Secrets Manager, RDS, ECS TODO)
```

## Quick start

```bash
cp .env.example .env        # then set NVE_API_KEY
make sqlc                   # generate internal/db (needed before build)
make up                     # db + ingester + api via docker compose
```

API:

```bash
curl localhost:8080/healthz
curl localhost:8080/stations
curl localhost:8080/stations/2.32.0/latest
curl "localhost:8080/stations/2.32.0/observations?parameter=1000&from=2026-06-20T00:00:00Z"
```

Parameters: `1000` discharge, `1001` water level, `1003` water temperature.

## Local development (no Docker)

```bash
docker compose up -d db     # just the database
make sqlc
make run-ingester           # in one shell
make run-api                # in another
```

## Regenerating DB code

`internal/db` is generated from `db/migrations` (schema) and `db/queries`:

```bash
make sqlc
```

## Infrastructure

The `terraform/` directory is an AWS skeleton: ECR, Secrets Manager (DB
password + NVE key), and an RDS Postgres instance. The ECS/Fargate services are
stubbed in `main.tf` — publish an image to ECR, then fill them in.

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
```
