.PHONY: dev web build test test-xray fmt

dev:
	go run ./cmd/server

web:
	npm --prefix web run build

build: web
	go build -o bin/sub-manager ./cmd/server

test:
	go test ./cmd/... ./internal/...
	npm --prefix web run typecheck

test-xray:
	XRAY_BIN="$${XRAY_BIN:-xray}" go test ./internal/protocol -run TestGeneratedOutboundWithOfficialXray -v

fmt:
	gofmt -w $$(find cmd internal -name '*.go')
	npm --prefix web run format
