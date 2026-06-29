# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ingester ./cmd/ingester
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=build /out/ingester /usr/local/bin/ingester
COPY --from=build /out/api /usr/local/bin/api
USER nonroot:nonroot
# Default to the ingester; docker-compose overrides `command:` to run the api.
CMD ["/usr/local/bin/ingester"]
