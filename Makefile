SHELL := /bin/bash

.PHONY: dev-api dev-web test build

dev-api:
	cd apps/api && go run ./cmd/api

dev-web:
	cd apps/web && npm run dev -- --host 0.0.0.0

test:
	cd apps/api && go test ./...
	cd apps/web && npm run check

e2e:
	docker compose -f docker-compose.e2e.yml up --build --abort-on-container-exit --exit-code-from e2e e2e

e2e-clean:
	docker compose -f docker-compose.e2e.yml down --volumes --remove-orphans

build:
	cd apps/api && go build ./cmd/api
	cd apps/web && npm run build
