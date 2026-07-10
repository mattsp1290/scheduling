SHELL := /bin/bash

.PHONY: dev-api dev-web test build

dev-api:
	cd apps/api && go run ./cmd/api

dev-web:
	cd apps/web && npm run dev -- --host 0.0.0.0

test:
	cd apps/api && go test ./...
	cd apps/web && npm run check

build:
	cd apps/api && go build ./cmd/api
	cd apps/web && npm run build
