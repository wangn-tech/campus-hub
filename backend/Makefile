SHELL := /bin/sh

CONFIG_FILE ?= configs/config.dev.yaml
COMPOSE_FILE := deploy/docker/docker-compose.yml

.PHONY: dev-up dev-down dev-logs run test vet fmt lint migrate-up migrate-down openapi compose-config integration

dev-up:
	docker compose -f $(COMPOSE_FILE) --env-file .env.example up -d mysql redis kafka elasticsearch minio mailpit

dev-down:
	docker compose -f $(COMPOSE_FILE) --env-file .env.example down

dev-logs:
	docker compose -f $(COMPOSE_FILE) --env-file .env.example logs -f

run:
	CAMPUSHUB_CONFIG_FILE=$(CONFIG_FILE) go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

lint: fmt vet

integration:
	./scripts/ci-integration.sh

migrate-up:
	go run ./cmd/migrate -direction up

migrate-down:
	go run ./cmd/migrate -direction down

openapi:
	@test -s api/openapi.yaml
	@echo "OpenAPI contract is present: api/openapi.yaml"

compose-config:
	docker compose -f $(COMPOSE_FILE) --env-file .env.example config --quiet
