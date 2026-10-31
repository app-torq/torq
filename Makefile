# torq — see specs/001-torq-core-platform/quickstart.md
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DATA_DIR ?= ./.data

.PHONY: setup generate dev dev-api dev-web build-web build check test e2e docker clean

setup:
	go mod download
	npm ci --prefix web

generate:
	sqlc generate

# Runs the Go API (:8080) and the Vite dev server (:5173, proxies /api) together.
dev:
	@trap 'kill 0' INT TERM EXIT; \
	TORQ_DATA_DIR=$(DATA_DIR) TORQ_LOG_FORMAT=text go run ./cmd/torq & \
	npm run dev --prefix web & \
	wait

build-web:
	npm run build --prefix web

build: build-web
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/torq ./cmd/torq

check:
	go vet ./...
	golangci-lint run ./...
	sqlc diff
	npm run typecheck --prefix web
	npm run lint --prefix web
	npm run i18n:check --prefix web

test:
	go test ./...
	npm test --prefix web -- --run

# End-to-end suite: starts bin/torq on a temp data dir, or targets TORQ_E2E_URL.
e2e: build
	npm run e2e --prefix web

docker:
	docker build --build-arg VERSION=$(VERSION) -t torq:$(VERSION) -t torq:latest .

clean:
	rm -rf bin web/dist/assets web/dist/index.html web/dist/favicon.svg
