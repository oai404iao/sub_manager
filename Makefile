.PHONY: dev web build test fmt

dev:
	go run ./cmd/server

web:
	npm --prefix web run build

build: web
	go build -o bin/sub-manager ./cmd/server

test:
	go test ./cmd/... ./internal/...
	npm --prefix web run typecheck

fmt:
	gofmt -w $$(find cmd internal -name '*.go')
	npm --prefix web run format
