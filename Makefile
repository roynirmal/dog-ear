.DEFAULT_GOAL := help

.PHONY: help install build up dev test

help:
	@printf '%s\n' 'make install  Download locked backend/frontend dependencies' 'make up       Build the frontend and serve everything from PocketBase on :8091' 'make dev      Backend on :8091 plus Next.js hot reload on :5174 (Ctrl+C stops both)' 'make test     Formatting, vet, backend tests, type check, unit tests and build'

install:
	cd backend && go mod download
	cd frontend && npm ci

# PocketBase serves the static export from backend/pb_public.
build:
	cd frontend && NEXT_TELEMETRY_DISABLED=1 npm run build
	rm -rf backend/pb_public && cp -R frontend/out backend/pb_public

up: build
	cd backend && go run . serve --http=127.0.0.1:8091

dev:
	@bash scripts/dev.sh

# Same checks as .github/workflows/ci.yml.
test:
	@test -z "$$(cd backend && gofmt -l .)" || { echo 'Run gofmt -w on:'; cd backend && gofmt -l .; exit 1; }
	cd backend && go vet ./... && go test -race ./...
	cd frontend && npx prettier --check app components lib && npm run typecheck && npm test && NEXT_TELEMETRY_DISABLED=1 npm run build
