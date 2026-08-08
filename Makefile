.PHONY: dev web build test test-xray lint fmt fmt-check vet ci docker-build release-artifacts release

VERSION ?= $(shell cat VERSION 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
OUTPUT ?= bin/sub-manager
IMAGE ?= ghcr.io/oai404iao/sub_manager
TAG ?= $(VERSION)

GO_LDFLAGS = -s -w \
	-X github.com/oai404iao/sub_manager/internal/version.Version=$(VERSION) \
	-X github.com/oai404iao/sub_manager/internal/version.Commit=$(COMMIT) \
	-X github.com/oai404iao/sub_manager/internal/version.BuildDate=$(BUILD_DATE)

dev:
	go run -ldflags "$(GO_LDFLAGS)" ./cmd/server

web:
	npm --prefix web run build

build: web
	mkdir -p "$(dir $(OUTPUT))"
	CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o "$(OUTPUT)" ./cmd/server

test:
	go test ./cmd/... ./internal/...
	npm --prefix web run typecheck

vet:
	go vet ./cmd/... ./internal/...

lint:
	npm --prefix web run lint

test-xray:
	XRAY_BIN="$${XRAY_BIN:-xray}" go test ./internal/protocol -run TestGeneratedOutboundWithOfficialXray -v

fmt:
	gofmt -w $$(find cmd internal -name '*.go')
	npm --prefix web run format

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || \
		(echo "Go files need formatting:"; gofmt -l cmd internal; exit 1)

ci: fmt-check vet test lint web

docker-build:
	docker build \
		--build-arg VERSION="$(VERSION)" \
		--build-arg COMMIT="$(COMMIT)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-t "$(IMAGE):$(TAG)" .

release-artifacts:
	./scripts/build-release.sh "$(VERSION)"

release:
	./scripts/release.sh "$(VERSION)"
